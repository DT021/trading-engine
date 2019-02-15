package engine

import (
	"alex/marketdata"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

type IBroker interface {
	Connect(errChan chan error, eventChan chan event, orderMutex *sync.Mutex)
	OnNewOrder(e *NewOrderEvent)
	OnCancelRequest(e *OrderCancelRequestEvent)
	OnReplaceRequest(e *OrderReplaceRequestEvent)
	IsSimulated() bool
	OnCandleClose(e *CandleCloseEvent)
	OnCandleOpen(e *CandleOpenEvent)
	OnTick(tick *marketdata.Tick)
}

type SimulatedBroker struct {
	errChan                chan error
	eventChan              chan event
	newOrders              map[string]*Order
	filledOrders           map[string]*Order
	canceledOrders         map[string]*Order
	confirmedOrders        map[string]*Order
	rejectedOrders         map[string]*Order
	allOrders              map[string]*Order
	delay                  int64
	hasQuotesAndTrades     bool
	strictLimitOrders      bool
	marketOpenUntilTime    TimeOfDay
	marketCloseUntilTime   TimeOfDay
	checkExecutionsOnTicks bool
	fraction               int64
	mpMutext               *sync.Mutex
}

func (b *SimulatedBroker) IsSimulated() bool {
	return true
}

func (b *SimulatedBroker) Connect(errChan chan error, eventChan chan event, orderMutex *sync.Mutex) {
	if errChan == nil {
		panic("Can't connect simulated broker. Error chan is nil. ")
	}
	if eventChan == nil {
		panic("Can't connect simulated broker. Event chan is nil. ")
	}
	b.errChan = errChan
	b.eventChan = eventChan

	b.filledOrders = make(map[string]*Order)
	b.canceledOrders = make(map[string]*Order)
	b.confirmedOrders = make(map[string]*Order)
	b.rejectedOrders = make(map[string]*Order)
	b.newOrders = make(map[string]*Order)
	b.allOrders = make(map[string]*Order)

	b.mpMutext = orderMutex

}

func (b *SimulatedBroker) OnNewOrder(e *NewOrderEvent) {

	if !e.LinkedOrder.isValid() {
		r := "Sim Broker: can't confirm order. Order is not valid"
		rejectEvent := OrderRejectedEvent{
			OrdId:     e.LinkedOrder.Id,
			Reason:    r,
			BaseEvent: be(e.Time, e.Symbol),
		}
		go b.newEvent(&rejectEvent)
		b.rejectedOrders[e.LinkedOrder.Id] = e.LinkedOrder
		b.allOrders[e.LinkedOrder.Id] = e.LinkedOrder
		return
	}

	if _, ok := b.allOrders[e.LinkedOrder.Id]; ok {
		r := "Sim Broker: can't confirm order. Order with this ID already exists on broker side"
		rejectEvent := OrderRejectedEvent{
			OrdId:     e.LinkedOrder.Id,
			Reason:    r,
			BaseEvent: be(e.Time, e.Symbol),
		}
		go b.newEvent(&rejectEvent)
		b.rejectedOrders[e.LinkedOrder.Id] = e.LinkedOrder

		return
	}

	b.allOrders[e.LinkedOrder.Id] = e.LinkedOrder
	b.newOrders[e.LinkedOrder.Id] = e.LinkedOrder
	confEvent := OrderConfirmationEvent{
		OrdId:     e.LinkedOrder.Id,
		BaseEvent: be(e.LinkedOrder.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
	}

	go b.newEvent(&confEvent)

}

func (b *SimulatedBroker) OnCancelRequest(e *OrderCancelRequestEvent) {
	err := b.validateOrderModificationRequest(e.OrdId, "cancel")
	if err != nil {
		go b.newError(err)
		return
	}
	orderCancelE := OrderCancelEvent{
		OrdId:     e.OrdId,
		BaseEvent: be(e.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
	}
	go b.newEvent(&orderCancelE)

}

func (b *SimulatedBroker) OnReplaceRequest(e *OrderReplaceRequestEvent) {
	err := b.validateOrderModificationRequest(e.OrdId, "replace")
	if err != nil {
		go b.newError(err)
		return
	}

	if math.IsNaN(e.NewPrice) || e.NewPrice == 0 {
		err := ErrInvalidRequestPrice{
			Price:   e.NewPrice,
			Message: fmt.Sprintf("Can't replace order: %v", e.OrdId),
			Caller:  "Sim Broker",
		}
		go b.newError(&err)
		return

	}

	replacedEvent := OrderReplacedEvent{
		OrdId:     e.OrdId,
		NewPrice:  e.NewPrice,
		BaseEvent: be(e.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
	}

	go b.newEvent(&replacedEvent)

}

func (b *SimulatedBroker) validateOrderModificationRequest(ordId string, modType string) error {
	if _, ok := b.confirmedOrders[ordId]; !ok {
		err := ErrOrderNotFoundInConfirmedMap{
			OrdId:   ordId,
			Message: fmt.Sprintf("Can't %v order.", modType),
			Caller:  "Sim Broker",
		}
		return &err

	}
	if b.confirmedOrders[ordId].State != ConfirmedOrder && b.confirmedOrders[ordId].State != PartialFilledOrder {
		err := ErrUnexpectedOrderState{
			OrdId:         ordId,
			ActualState:   string(b.confirmedOrders[ordId].State),
			ExpectedState: string(ConfirmedOrder) + "," + string(PartialFilledOrder),
			Message:       fmt.Sprintf("Can't %v order.", modType),
			Caller:        "Sim Broker",
		}
		return &err

	}

	return nil
}

func (b *SimulatedBroker) OnCandleOpen(e *CandleOpenEvent) {
	if b.checkExecutionsOnTicks {
		return
	}

	//Todo

}

func (b *SimulatedBroker) OnCandleClose(e *CandleCloseEvent) {
	if b.checkExecutionsOnTicks {
		return
	}
	//todo

}

func (b *SimulatedBroker) OnTick(tick *marketdata.Tick) {
	if !b.checkExecutionsOnTicks {
		return
	}
	if !b.tickIsValid(tick) {
		err := ErrBrokenTick{
			Tick:    *tick,
			Message: "Got in OnTick",
			Caller:  "Sim Broker",
		}

		go b.newError(&err)
		return

	}

	if len(b.confirmedOrders) == 0 {
		return
	}

	for _, o := range b.confirmedOrders {
		if o.Symbol == tick.Symbol {
			b.checkOrderExecutionOnTick(o, tick)
		}

	}

}

func (b *SimulatedBroker) tickIsValid(tick *marketdata.Tick) bool {
	if tick.HasQuote {
		if math.IsNaN(tick.BidPrice) || math.IsNaN(tick.AskPrice) || tick.BidPrice == 0 || tick.AskPrice == 0 {
			return false
		}
	}

	if tick.HasTrade {
		if math.IsNaN(tick.LastPrice) || tick.LastPrice == 0 || tick.LastSize == 0 {
			return false
		}
	}
	return true
}

func (b *SimulatedBroker) checkOrderExecutionOnTick(order *Order, tick *marketdata.Tick) {

	switch order.Type {
	case MarketOrder:
		b.checkOnTickMarket(order, tick)
		return
	case LimitOrder:
		b.checkOnTickLimit(order, tick)
		return
	case StopOrder:
		b.checkOnTickStop(order, tick)
		return
	case LimitOnClose:
		b.checkOnTickLOC(order, tick)
		return
	case LimitOnOpen:
		b.checkOnTickLOO(order, tick)
		return
	case MarketOnOpen:
		b.checkOnTickMOO(order, tick)
		return
	case MarketOnClose:
		b.checkOnTickMOC(order, tick)
		return
	default:
		err := ErrUnknownOrderType{
			OrdId:   order.Id,
			Message: "found order with type: " + string(order.Type),
			Caller:  "Sim Broker",
		}
		go b.newError(&err)
	}

}

func (b *SimulatedBroker) checkOnTickLOO(order *Order, tick *marketdata.Tick) {
	err := b.validateOrderForExecution(order, LimitOnOpen)
	if err != nil {
		go b.newError(err)
		return
	}

	if !tick.IsOpening {
		if b.marketOpenUntilTime.Before(tick.Datetime) {
			//b.updateCanceledOrders(order)
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
			}
			go b.newEvent(&cancelE)
		}
		return
	}

	b.checkOnTickLimitAuction(order, tick)

}

func (b *SimulatedBroker) checkOnTickLOC(order *Order, tick *marketdata.Tick) {
	err := b.validateOrderForExecution(order, LimitOnClose)
	if err != nil {
		go b.newError(err)
		return
	}

	if !tick.IsClosing {
		if b.marketCloseUntilTime.Before(tick.Datetime) {
			//b.updateCanceledOrders(order)
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			go b.newEvent(&cancelE)
		}
		return
	}

	b.checkOnTickLimitAuction(order, tick)

}

func (b *SimulatedBroker) checkOnTickLimitAuction(order *Order, tick *marketdata.Tick) {
	switch order.Side {
	case OrderSell:
		if tick.LastPrice < order.Price {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			//b.updateCanceledOrders(order)
			go b.newEvent(&cancelE)
			return
		}

	case OrderBuy:
		if tick.LastPrice > order.Price {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			//b.updateCanceledOrders(order)
			go b.newEvent(&cancelE)
			return
		}

	default:
		err := ErrUnknownOrderSide{
			OrdId:   order.Id,
			Message: "From checkOnTickLimitAuction",
			Caller:  "Sim Broker",
		}
		go b.newError(&err)
		return

	}

	if tick.LastPrice == order.Price && b.strictLimitOrders {
		cancelE := OrderCancelEvent{
			OrdId:     order.Id,
			BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
		}
		//b.updateCanceledOrders(order)
		go b.newEvent(&cancelE)
		return
	}

	execQty := order.Qty
	if execQty > int(tick.LastSize) {
		execQty = int(tick.LastSize)
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       execQty,
		BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
	}

	go b.newEvent(&fillE)

	if execQty < order.Qty {
		cancelE := OrderCancelEvent{
			OrdId:     order.Id,
			BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
		}
		time.Sleep(time.Duration(b.delay/b.fraction) * time.Millisecond)
		go b.newEvent(&cancelE)
	}

}

func (b *SimulatedBroker) checkOnTickMOO(order *Order, tick *marketdata.Tick) {
	err := b.validateOrderForExecution(order, MarketOnOpen)
	if err != nil {
		go b.newError(err)
		return
	}

	if !tick.IsOpening {
		return
	}

	if !tick.HasTrade {
		return
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       order.Qty,
		BaseEvent: be(tick.Datetime, order.Symbol),
	}
	go b.newEvent(&fillE)
	return

}

func (b *SimulatedBroker) checkOnTickMOC(order *Order, tick *marketdata.Tick) {
	//Todo подумать над реализацией когда отркрывающего тика вообще нет
	err := b.validateOrderForExecution(order, MarketOnClose)
	if err != nil {
		go b.newError(err)
		return
	}

	if !tick.IsClosing {
		return
	}

	if !tick.HasTrade {
		return
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       order.Qty,
		BaseEvent: be(tick.Datetime, order.Symbol),
	}

	go b.newEvent(&fillE)
	return

}

func (b *SimulatedBroker) checkOnTickLimit(order *Order, tick *marketdata.Tick) {

	err := b.validateOrderForExecution(order, LimitOrder)
	if err != nil {
		go b.newError(err)
		return
	}

	if !tick.HasTrade {
		return
	}

	if math.IsNaN(tick.LastPrice) {
		return
	}
	lvsQty := order.Qty - order.ExecQty
	if lvsQty <= 0 {
		go b.newError(errors.New("Sim broker: Lvs qty is zero or less. Nothing to execute. "))
		return
	}
	switch order.Side {
	case OrderSell:
		if tick.LastPrice > order.Price {
			qty := lvsQty
			if tick.LastSize < int64(qty) {
				qty = int(tick.LastSize)
			}

			fillE := OrderFillEvent{
				OrdId:     order.Id,
				Price:     order.Price,
				Qty:       qty,
				BaseEvent: be(tick.Datetime, order.Symbol),
			}

			go b.newEvent(&fillE)
			return

		} else {
			if tick.LastPrice == order.Price && !b.strictLimitOrders {
				qty := lvsQty
				if tick.LastSize < int64(qty) {
					qty = int(tick.LastSize)
				}

				fillE := OrderFillEvent{
					OrdId:     order.Id,
					Price:     order.Price,
					Qty:       qty,
					BaseEvent: be(tick.Datetime, order.Symbol),
				}

				go b.newEvent(&fillE)
				return
			} else {
				return
			}
		}

	case OrderBuy:
		if tick.LastPrice < order.Price {
			qty := lvsQty
			if tick.LastSize < int64(qty) {
				qty = int(tick.LastSize)
			}

			fillE := OrderFillEvent{
				OrdId:     order.Id,
				Price:     order.Price,
				Qty:       qty,
				BaseEvent: be(tick.Datetime, order.Symbol),
			}

			go b.newEvent(&fillE)
			return

		} else {
			if tick.LastPrice == order.Price && !b.strictLimitOrders {
				qty := lvsQty
				if tick.LastSize < int64(qty) {
					qty = int(tick.LastSize)
				}

				fillE := OrderFillEvent{
					OrdId:     order.Id,
					Price:     order.Price,
					Qty:       qty,
					BaseEvent: be(tick.Datetime, order.Symbol),
				}

				go b.newEvent(&fillE)
				return
			} else {
				return
			}
		}
	default:
		go b.newError(errors.New("Sim broker: can't check fill for order. Unknown side. "))
		return

	}

}

func (b *SimulatedBroker) checkOnTickStop(order *Order, tick *marketdata.Tick) {
	err := b.validateOrderForExecution(order, StopOrder)
	if err != nil {
		go b.newError(err)
		return
	}

	if !tick.HasTrade {
		return
	}

	switch order.Side {
	case OrderSell:
		if tick.LastPrice > order.Price {
			return
		}
		price := tick.LastPrice
		lvsQty := order.Qty - order.ExecQty
		qty := lvsQty
		if int(tick.LastSize) < qty {
			qty = int(tick.LastSize)
		}
		if tick.HasQuote {
			price = tick.BidPrice
			qty = lvsQty
		}
		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     price,
			Qty:       qty,
			BaseEvent: be(tick.Datetime, order.Symbol),
		}

		go b.newEvent(&fillE)
		return

	case OrderBuy:
		if tick.LastPrice < order.Price {
			return
		}
		price := tick.LastPrice
		lvsQty := order.Qty - order.ExecQty
		qty := lvsQty
		if int(tick.LastSize) < qty {
			qty = int(tick.LastSize)
		}
		if tick.HasQuote {
			price = tick.AskPrice
			qty = lvsQty
		}
		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     price,
			Qty:       qty,
			BaseEvent: be(tick.Datetime, order.Symbol),
		}

		go b.newEvent(&fillE)
		return

	default:
		err := ErrUnknownOrderSide{
			OrdId:   order.Id,
			Message: "Got in checkOnTickStop",
			Caller:  "Sim Broker",
		}
		go b.newError(&err)
		return
	}

}

func (b *SimulatedBroker) checkOnTickMarket(order *Order, tick *marketdata.Tick) {
	err := b.validateOrderForExecution(order, MarketOrder)
	if err != nil {
		go b.newError(err)
		return
	}

	if b.hasQuotesAndTrades && !tick.HasQuote {
		return
	}
	if !b.hasQuotesAndTrades && tick.HasQuote {
		go b.newError(errors.New("Sim Broker: broker doesn't expect quotes. Only trades. "))
		return
	}

	if b.hasQuotesAndTrades {
		qty := 0
		price := math.NaN()
		lvsQty := order.Qty - order.ExecQty

		if order.Side == OrderBuy {
			if int64(lvsQty) > tick.AskSize { //Todo Smell
				qty = int(tick.AskSize)
			} else {
				qty = lvsQty
			}

			price = tick.AskPrice

		} else { //Short order logic + sanity check for Side issues
			if order.Side != OrderSell {
				go b.newError(errors.New("Sim Broker: unknown order side: " + string(order.Side)))
				return
			}

			if int64(lvsQty) > tick.BidSize { //Todo Smell
				qty = int(tick.BidSize)
			} else {
				qty = lvsQty
			}

			price = tick.BidPrice
		}
		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     price,
			Qty:       qty,
			BaseEvent: be(tick.Datetime, order.Symbol),
		}

		go b.newEvent(&fillE)

	} else { //If broker accepts only trades without quotes
		if !tick.HasTrade {
			go b.newError(errors.New("Sim Broker: tick doesn't contain trade. "))
			return
		}

		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     tick.LastPrice,
			Qty:       order.Qty,
			BaseEvent: be(tick.Datetime, order.Symbol),
		}

		go b.newEvent(&fillE)
	}

}

//validateOrderForExecution checks if order is valid and can be filled. Returns nil if order is valid
//or error in other cases
func (b *SimulatedBroker) validateOrderForExecution(order *Order, expectedType OrderType) error {
	if !order.isValid() {
		err := ErrInvalidOrder{
			OrdId:   order.Id,
			Message: "Got in checkOnTickLimit",
			Caller:  "Sim Broker",
		}

		return &err
	}

	if order.Type != expectedType {
		err := ErrUnexpectedOrderType{
			OrdId:        order.Id,
			ActualType:   string(order.Type),
			ExpectedType: string(expectedType),
			Message:      "Got in checkOnTickLimit",
			Caller:       "Sim Broker",
		}
		return &err
	}

	if order.State != ConfirmedOrder && order.State != PartialFilledOrder {

		err := ErrUnexpectedOrderState{
			OrdId:         order.Id,
			ActualState:   string(order.State),
			ExpectedState: string(ConfirmedOrder) + "," + string(PartialFilledOrder),
			Message:       "Got in checkOnTickLimit",
			Caller:        "Sim Broker",
		}
		return &err
	}

	return nil
}

func (b *SimulatedBroker) newEvent(e event) {
	if b.eventChan == nil {
		panic("Simulated broker event chan is nil")
	}
	time.Sleep(time.Duration(b.delay/b.fraction) * time.Millisecond)
	b.eventChan <- e
	switch i := e.(type) {


	case *OrderConfirmationEvent:
		b.mpMutext.Lock()
		ord, ok := b.newOrders[i.OrdId]
		if !ok {
			panic("Confirmation of not existing order")
		}

		b.confirmedOrders[i.OrdId] = ord
		delete(b.newOrders, i.OrdId)
		b.mpMutext.Unlock()

	case *OrderCancelEvent:
		b.mpMutext.Lock()
		ord, ok := b.confirmedOrders[i.OrdId]
		if !ok {
			msg := fmt.Sprintf("Can't find order %v in confirmed map to cancel it. ", i.OrdId)
			panic(msg)
		}
		b.canceledOrders[i.OrdId] = ord
		delete(b.confirmedOrders, i.OrdId)
		b.mpMutext.Unlock()

	case *OrderFillEvent:
		b.mpMutext.Lock()
		ord, ok := b.confirmedOrders[i.OrdId]
		if !ok {
			msg := fmt.Sprintf("Can't find order %v in confirmed map to fill it. ", i.OrdId)
			panic(msg)
		}
		execQty := i.Qty
		if execQty == ord.Qty-ord.ExecQty {
			b.filledOrders[i.OrdId] = ord
			delete(b.confirmedOrders, i.OrdId)
		}
		b.mpMutext.Unlock()

	}

}

func (b *SimulatedBroker) newError(e error) {
	if b.errChan == nil {
		panic("Simulated broker error chan is nil")
	}
	b.errChan <- e
}
