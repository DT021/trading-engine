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
	IsSimulated() bool
	Connect(err chan error, symbols []string)
	OnNewOrder(e *NewOrderEvent)
	OnCancelRequest(e *OrderCancelRequestEvent)
	OnReplaceRequest(e *OrderReplaceRequestEvent)

	OnCandleClose(e *CandleCloseEvent)
	OnCandleOpen(e *CandleOpenEvent)
	OnTick(tick *marketdata.Tick)
	GetAllChannelsMap() map[string]chan event
}

type SimBrokerOrder struct {
	*Order
	BrokerState   OrderState
	StateUpdTime  time.Time
	BrokerExecQty int
}

type SimBroker struct {
	delay                  int64
	checkExecutionsOnTicks bool
	hasQuotesAndTrades     bool
	strictLimitOrders      bool
	marketOpenUntilTime    TimeOfDay
	marketCloseUntilTime   TimeOfDay
	workers                map[string]*SimulatedBrokerWorker
}

func (b *SimBroker) Connect(err chan error, symbols []string) {
	if len(symbols) == 0 {
		panic("No symbols specified")
	}

	b.workers = make(map[string]*SimulatedBrokerWorker)

	for _, s := range symbols {
		bw := SimulatedBrokerWorker{
			symbol:               s,
			errChan:              err,
			delay:                b.delay,
			hasQuotesAndTrades:   b.hasQuotesAndTrades,
			strictLimitOrders:    b.strictLimitOrders,
			marketOpenUntilTime:  b.marketCloseUntilTime,
			marketCloseUntilTime: b.marketCloseUntilTime,
			mpMutext:             &sync.RWMutex{},
			orders:               make(map[string]*SimBrokerOrder),
		}

		b.workers[s] = &bw

	}
}

func (b *SimBroker) getWorker(symbol string) *SimulatedBrokerWorker {
	w, ok := b.workers[symbol]
	if !ok {
		panic("Symbol not found in sim broker")
	}
	return w
}

func (b *SimBroker) GetAllChannelsMap() map[string]chan event {
	allChans := make(map[string]chan event)
	for _, w := range b.workers {
		allChans[w.symbol] = w.eventChan
	}

	return allChans
}

func (b *SimBroker) IsSimulated() bool {
	return true
}

func (b *SimBroker) OnNewOrder(e *NewOrderEvent) {
	w := b.getWorker(e.Symbol)
	go w.OnNewOrder(e)
}

func (b *SimBroker) OnCancelRequest(e *OrderCancelRequestEvent) {
	w := b.getWorker(e.Symbol)
	go w.OnCancelRequest(e)

}

func (b *SimBroker) OnReplaceRequest(e *OrderReplaceRequestEvent) {
	w := b.getWorker(e.Symbol)
	go w.OnReplaceRequest(e)

}

func (b *SimBroker) OnTick(tick *marketdata.Tick) {
	if !b.checkExecutionsOnTicks {
		return
	}

	w := b.getWorker(tick.Symbol)
	go w.OnTick(tick)

}

func (b *SimBroker) OnCandleClose(e *CandleCloseEvent) {
	panic("Not implemented")

}

func (b *SimBroker) OnCandleOpen(e *CandleOpenEvent) {
	panic("Not implemented")
}

type SimulatedBrokerWorker struct {
	symbol    string
	errChan   chan error
	eventChan chan event

	delay                int64
	hasQuotesAndTrades   bool
	strictLimitOrders    bool
	marketOpenUntilTime  TimeOfDay
	marketCloseUntilTime TimeOfDay

	mpMutext *sync.RWMutex
	orders   map[string]*SimBrokerOrder
}

func (b *SimulatedBrokerWorker) OnNewOrder(e *NewOrderEvent) {
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()

	if !e.LinkedOrder.isValid() {
		r := "Sim Broker: can't confirm order. Order is not valid"
		rejectEvent := OrderRejectedEvent{
			OrdId:     e.LinkedOrder.Id,
			Reason:    r,
			BaseEvent: be(e.LinkedOrder.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
		}
		b.orders[e.LinkedOrder.Id] = &SimBrokerOrder{
			Order:         e.LinkedOrder,
			BrokerState:   RejectedOrder,
			BrokerExecQty: 0,
			StateUpdTime:  rejectEvent.getTime(),
		}
		b.newEvent(&rejectEvent)

		return
	}

	if _, ok := b.orders[e.LinkedOrder.Id]; ok {
		r := "Sim Broker: can't confirm order. Order with this ID already exists on broker side"
		rejectEvent := OrderRejectedEvent{
			OrdId:     e.LinkedOrder.Id,
			Reason:    r,
			BaseEvent: be(e.LinkedOrder.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
		}
		b.newEvent(&rejectEvent)

		return
	}

	confEvent := OrderConfirmationEvent{
		OrdId:     e.LinkedOrder.Id,
		BaseEvent: be(e.LinkedOrder.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
	}

	b.orders[e.LinkedOrder.Id] = &SimBrokerOrder{
		Order:         e.LinkedOrder,
		BrokerState:   ConfirmedOrder,
		BrokerExecQty: 0,
		StateUpdTime:  confEvent.getTime(),
	}

	b.newEvent(&confEvent)

}

func (b *SimulatedBrokerWorker) OnCancelRequest(e *OrderCancelRequestEvent) {
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()

	err := b.validateOrderModificationRequest(e.OrdId, "cancel")
	if err != nil {
		go b.newError(err)
		return
	}
	orderCancelE := OrderCancelEvent{
		OrdId:     e.OrdId,
		BaseEvent: be(e.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
	}
	b.newEvent(&orderCancelE)

}

func (b *SimulatedBrokerWorker) OnReplaceRequest(e *OrderReplaceRequestEvent) {
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

func (b *SimulatedBrokerWorker) validateOrderModificationRequest(ordId string, modType string) error {

	if _, ok := b.orders[ordId]; !ok {
		err := ErrOrderNotFoundInOrdersMap{
			OrdId:   ordId,
			Message: fmt.Sprintf("Can't %v order.", modType),
			Caller:  "Sim Broker",
		}
		return &err

	}
	if b.orders[ordId].State != ConfirmedOrder && b.orders[ordId].State != PartialFilledOrder {
		err := ErrUnexpectedOrderState{
			OrdId:         ordId,
			ActualState:   string(b.orders[ordId].State),
			ExpectedState: string(ConfirmedOrder) + "," + string(PartialFilledOrder),
			Message:       fmt.Sprintf("Can't %v order.", modType),
			Caller:        "Sim Broker",
		}
		return &err

	}

	return nil
}

func (b *SimulatedBrokerWorker) OnCandleOpen(e *CandleOpenEvent) {

	//Todo

}

func (b *SimulatedBrokerWorker) OnCandleClose(e *CandleCloseEvent) {

	//todo

}

func (b *SimulatedBrokerWorker) OnTick(tick *marketdata.Tick) {

	if !b.tickIsValid(tick) {
		err := ErrBrokenTick{
			Tick:    *tick,
			Message: "Got in OnTick",
			Caller:  "Sim Broker",
		}

		go b.newError(&err)
		return

	}
	b.mpMutext.Lock()

	if len(b.orders) == 0 {
		b.mpMutext.Unlock()
		return
	}

	var genEvents []event

	for _, o := range b.orders {
		if o.Symbol == tick.Symbol && (o.BrokerState == ConfirmedOrder || o.BrokerState == PartialFilledOrder) {
			e := b.checkOrderExecutionOnTick(o, tick)
			if e != nil {
				genEvents = append(genEvents, e)
			}
		}
	}

	if len(genEvents) > 0 {
		for _, e := range genEvents {
			b.newEvent(e)
		}
	}

	b.mpMutext.Unlock()

}

func (b *SimulatedBrokerWorker) tickIsValid(tick *marketdata.Tick) bool {
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

func (b *SimulatedBrokerWorker) checkOrderExecutionOnTick(orderSim *SimBrokerOrder, tick *marketdata.Tick) event {

	err := b.validateOrderForExecution(orderSim, orderSim.Type)
	if err != nil {
		go b.newError(err)
		return nil
	}

	switch orderSim.Type {

	case MarketOrder:
		b.checkOnTickMarket(orderSim, tick)
		return nil
	case LimitOrder:
		e := b.checkOnTickLimit(orderSim, tick)
		return e
	case StopOrder:
		b.checkOnTickStop(orderSim, tick)
		return nil
	case LimitOnClose:
		b.checkOnTickLOC(orderSim, tick)
		return nil
	case LimitOnOpen:
		b.checkOnTickLOO(orderSim, tick)
		return nil
	case MarketOnOpen:
		b.checkOnTickMOO(orderSim, tick)
		return nil
	case MarketOnClose:
		b.checkOnTickMOC(orderSim, tick)
		return nil
	default:
		err := ErrUnknownOrderType{
			OrdId:   orderSim.Id,
			Message: "found order with type: " + string(orderSim.Type),
			Caller:  "Sim Broker",
		}
		go b.newError(&err)
	}

	return nil

}

func (b *SimulatedBrokerWorker) checkOnTickLOO(order *SimBrokerOrder, tick *marketdata.Tick) {

	if !tick.IsOpening {
		if b.marketOpenUntilTime.Before(tick.Datetime) {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
			}
			b.newEvent(&cancelE)
		}
		return
	}

	b.checkOnTickLimitAuction(order, tick)

}

func (b *SimulatedBrokerWorker) checkOnTickLOC(order *SimBrokerOrder, tick *marketdata.Tick) {
	if !tick.IsClosing {
		if b.marketCloseUntilTime.Before(tick.Datetime) {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			b.newEvent(&cancelE)
		}
		return
	}

	b.checkOnTickLimitAuction(order, tick)

}

func (b *SimulatedBrokerWorker) checkOnTickLimitAuction(order *SimBrokerOrder, tick *marketdata.Tick) {
	switch order.Side {
	case OrderSell:
		if tick.LastPrice < order.Price {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			b.newEvent(&cancelE)
			return
		}

	case OrderBuy:
		if tick.LastPrice > order.Price {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			b.newEvent(&cancelE)
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
		b.newEvent(&cancelE)
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

	b.newEvent(&fillE)

	if execQty < order.Qty {
		cancelE := OrderCancelEvent{
			OrdId:     order.Id,
			BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
		}

		b.newEvent(&cancelE)
	}

}

func (b *SimulatedBrokerWorker) checkOnTickMOO(order *SimBrokerOrder, tick *marketdata.Tick) {

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
	b.newEvent(&fillE)
	return

}

func (b *SimulatedBrokerWorker) checkOnTickMOC(order *SimBrokerOrder, tick *marketdata.Tick) {
	//Todo подумать над реализацией когда отркрывающего тика вообще нет
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

	b.newEvent(&fillE)
	return

}

func (b *SimulatedBrokerWorker) checkOnTickLimit(order *SimBrokerOrder, tick *marketdata.Tick) event {

	if !tick.HasTrade {
		return nil
	}

	if math.IsNaN(tick.LastPrice) {
		return nil
	}

	lvsQty := order.Qty - order.BrokerExecQty
	if lvsQty <= 0 {
		go b.newError(errors.New("Sim broker: Lvs qty is zero or less. Nothing to execute. "))
		return nil
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
			return &fillE

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

				return &fillE
			} else {
				return nil
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

			return &fillE

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

				return &fillE
			} else {
				return nil
			}
		}
	default:
		go b.newError(errors.New("Sim broker: can't check fill for order. Unknown side. "))
		return nil

	}

}

func (b *SimulatedBrokerWorker) checkOnTickStop(order *SimBrokerOrder, tick *marketdata.Tick) {
	if !tick.HasTrade {
		return
	}

	switch order.Side {
	case OrderSell:
		if tick.LastPrice > order.Price {
			return
		}
		price := tick.LastPrice
		lvsQty := order.Qty - order.BrokerExecQty
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

		b.newEvent(&fillE)
		return

	case OrderBuy:
		if tick.LastPrice < order.Price {
			return
		}
		price := tick.LastPrice
		lvsQty := order.Qty - order.BrokerExecQty
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

		b.newEvent(&fillE)
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

func (b *SimulatedBrokerWorker) checkOnTickMarket(order *SimBrokerOrder, tick *marketdata.Tick) {

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
		lvsQty := order.Qty - order.BrokerExecQty

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

		b.newEvent(&fillE)

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

		b.newEvent(&fillE)
	}

}

//validateOrderForExecution checks if order is valid and can be filled. Returns nil if order is valid
//or error in other cases
func (b *SimulatedBrokerWorker) validateOrderForExecution(order *SimBrokerOrder, expectedType OrderType) error {
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

	if order.BrokerState != ConfirmedOrder && order.BrokerState != PartialFilledOrder {

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

func (b *SimulatedBrokerWorker) newEvent(e event) {
	if b.eventChan == nil {
		panic("Simulated broker event chan is nil")
	}

	switch i := e.(type) {

	case *OrderConfirmationEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			panic("Confirmation of not existing order")
		}
		ord.BrokerState = ConfirmedOrder

	case *OrderCancelEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			msg := fmt.Sprintf("Can't find order %v in confirmed map to cancel it. ", i.OrdId)
			panic(msg)
		}

		ord.BrokerState = CanceledOrder

	case *OrderFillEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			msg := fmt.Sprintf("Can't find order %v in confirmed map to fill it. ", i.OrdId)
			panic(msg)
		}
		execQty := i.Qty

		if execQty == ord.Qty-ord.BrokerExecQty {
			ord.BrokerState = FilledOrder
		} else {
			if execQty > ord.Qty-ord.BrokerExecQty {
				panic("Large qty")
			}
			ord.BrokerState = PartialFilledOrder
		}

		ord.BrokerExecQty += i.Qty

	case *OrderRejectedEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			panic("Confirmation of not existing order")
		}
		ord.BrokerState = RejectedOrder

	}

	go func() {
		b.eventChan <- e
	}()

}

func (b *SimulatedBrokerWorker) newError(e error) {
	if b.errChan == nil {
		panic("Simulated broker error chan is nil")
	}
	b.errChan <- e
}
