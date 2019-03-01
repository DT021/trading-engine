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
	Connect()
	Disconnect()
	Init(errChan chan error, events chan event, symbols []string)
	IsSimulated() bool
	OnEvent(e event)
}

type simBrokerOrder struct {
	*Order
	BrokerState   OrderState
	StateUpdTime  time.Time
	BrokerExecQty int64
}

type SimBroker struct {
	delay                  int64
	checkExecutionsOnTicks bool
	strictLimitOrders      bool
	marketOpenUntilTime    TimeOfDay
	marketCloseUntilTime   TimeOfDay
	workers                map[string]*simBrokerWorker
}

func (b *SimBroker) Connect() {
	fmt.Println("SimBroker connected")
}

func (b *SimBroker) Disconnect() {
	fmt.Println("SimBroker connected")
}

func (b *SimBroker) Init(errChan chan error, events chan event, symbols []string) {
	if len(symbols) == 0 {
		panic("No symbols specified")
	}
	b.workers = make(map[string]*simBrokerWorker)

	for _, s := range symbols {
		bw := simBrokerWorker{
			symbol:               s,
			errChan:              errChan,
			events:               events,
			terminationChan:      make(chan struct{}),
			delay:                b.delay,
			strictLimitOrders:    b.strictLimitOrders,
			marketOpenUntilTime:  b.marketCloseUntilTime,
			marketCloseUntilTime: b.marketCloseUntilTime,
			mpMutext:             &sync.RWMutex{},
			orders:               make(map[string]*simBrokerOrder),
		}
		b.workers[s] = &bw

	}
}

func (b *SimBroker) OnEvent(e event) {
	w := b.workers[e.getSymbol()]
	b.proxyEvent(w, e)
}

func (b *SimBroker) proxyEvent(w *simBrokerWorker, e event) {
	switch i := e.(type) {
	case *NewOrderEvent:
		w.onNewOrder(i)
	case *OrderCancelRequestEvent:
		w.onCancelRequest(i)
	case *OrderReplaceRequestEvent:
		w.onReplaceRequest(i)
	case *NewTickEvent:
		w.onTick(i.Tick)
	default:
		panic("Unexpected event time in broker: " + e.getName())
	}
}

func (b *SimBroker) IsSimulated() bool {
	return true
}

//*******simBrokerWorker**********************************************
type simBrokerWorker struct {
	symbol            string
	errChan           chan error
	events            chan event
	eventReceivedChan chan event

	terminationChan      chan struct{}
	delay                int64
	strictLimitOrders    bool
	marketOpenUntilTime  TimeOfDay
	marketCloseUntilTime TimeOfDay
	mpMutext             *sync.RWMutex
	orders               map[string]*simBrokerOrder
}

func (b *simBrokerWorker) onNewOrder(e *NewOrderEvent) {
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()

	if !e.LinkedOrder.isValid() {
		r := "Sim Broker: can't confirm order. Order is not valid"
		rejectEvent := OrderRejectedEvent{
			OrdId:     e.LinkedOrder.Id,
			Reason:    r,
			BaseEvent: be(e.LinkedOrder.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
		}
		b.orders[e.LinkedOrder.Id] = &simBrokerOrder{
			Order:         e.LinkedOrder,
			BrokerState:   RejectedOrder,
			BrokerExecQty: 0,
			StateUpdTime:  rejectEvent.getTime(),
		}
		b.executeBrokerEvent(&rejectEvent)
		return
	}

	if _, ok := b.orders[e.LinkedOrder.Id]; ok {
		r := "Sim Broker: can't confirm order. Order with this ID already exists on broker side"
		rejectEvent := OrderRejectedEvent{
			OrdId:     e.LinkedOrder.Id,
			Reason:    r,
			BaseEvent: be(e.LinkedOrder.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
		}
		b.executeBrokerEvent(&rejectEvent)

		return
	}

	confEvent := OrderConfirmationEvent{
		OrdId:     e.LinkedOrder.Id,
		BaseEvent: be(e.LinkedOrder.Time.Add(time.Duration(b.delay)*time.Millisecond), e.Symbol),
	}

	b.orders[e.LinkedOrder.Id] = &simBrokerOrder{
		Order:         e.LinkedOrder,
		BrokerState:   NewOrder,
		BrokerExecQty: 0,
		StateUpdTime:  confEvent.getTime(),
	}

	b.executeBrokerEvent(&confEvent)

}

func (b *simBrokerWorker) onCancelRequest(e *OrderCancelRequestEvent) {
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()
	newEvTime := e.getTime().Add(time.Duration(b.delay) * time.Millisecond)

	if _, ok := b.orders[e.OrdId]; !ok {
		e := OrderCancelRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Broker can't find order with ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	if b.orders[e.OrdId].BrokerState == CanceledOrder {
		e := OrderCancelRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Order is already canceled ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	if b.orders[e.OrdId].BrokerState == NewOrder {
		e := OrderCancelRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Order is not confirmed yet ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	if b.orders[e.OrdId].BrokerState == FilledOrder {
		e := OrderCancelRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Order is already filled ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	orderCancelE := OrderCancelEvent{
		OrdId:     e.OrdId,
		BaseEvent: be(newEvTime, e.Symbol),
	}
	b.executeBrokerEvent(&orderCancelE)

}

func (b *simBrokerWorker) onReplaceRequest(e *OrderReplaceRequestEvent) {
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()
	newEvTime := e.getTime().Add(time.Duration(b.delay) * time.Millisecond)

	if _, ok := b.orders[e.OrdId]; !ok {
		e := OrderReplaceRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Broker can't find order with ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	if b.orders[e.OrdId].BrokerState == CanceledOrder {
		e := OrderReplaceRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Order is already canceled ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	if b.orders[e.OrdId].BrokerState == NewOrder {
		e := OrderReplaceRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Order is not confirmed yet ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	if b.orders[e.OrdId].BrokerState == FilledOrder {
		e := OrderReplaceRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    "Order is already filled ID: " + e.OrdId,
		}
		b.executeBrokerEvent(&e)
		return
	}

	if math.IsNaN(e.NewPrice) || e.NewPrice == 0 {
		e := OrderReplaceRejectEvent{
			BaseEvent: be(newEvTime, e.Symbol),
			OrdId:     e.OrdId,
			Reason:    fmt.Sprintf("Replace price %v is not valid", e.NewPrice),
		}
		b.executeBrokerEvent(&e)
		return
	}

	replacedEvent := OrderReplacedEvent{
		OrdId:     e.OrdId,
		NewPrice:  e.NewPrice,
		BaseEvent: be(newEvTime, e.Symbol),
	}
	b.executeBrokerEvent(&replacedEvent)
}

func (b *simBrokerWorker) checkModificationForReject(ordId string) error {

	if _, ok := b.orders[ordId]; !ok {
		err := ErrOrderNotFoundInOrdersMap{
			OrdId:   ordId,
			Message: fmt.Sprintf("Can't %v order.", "re"),
			Caller:  "Sim Broker",
		}
		return &err

	}

	if b.orders[ordId].BrokerState != ConfirmedOrder && b.orders[ordId].BrokerState != PartialFilledOrder {
		err := ErrUnexpectedOrderState{
			OrdId:         ordId,
			ActualState:   string(b.orders[ordId].State),
			ExpectedState: string(ConfirmedOrder) + "," + string(PartialFilledOrder),
			Message:       fmt.Sprintf("Can't %v order.", "re"),
			Caller:        "Sim Broker",
		}
		return &err

	}

	return nil
}

func (b *simBrokerWorker) onCandleOpen(e *CandleOpenEvent) {

	//Todo

}

func (b *simBrokerWorker) onCandleClose(e *CandleCloseEvent) {

	//todo

}

func (b *simBrokerWorker) onTick(tick *marketdata.Tick) {

	if !b.tickIsValid(tick) {
		err := ErrBrokenTick{
			Tick:    *tick,
			Message: "Got in onTick",
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
			if o.StateUpdTime.After(tick.Datetime) {
				continue
			}
			e := b.checkOrderExecutionOnTick(o, tick)
			if e != nil {
				genEvents = append(genEvents, e...)
			}
		}
	}

	if len(genEvents) > 0 {
		for _, e := range genEvents {
			b.executeBrokerEvent(e)
		}
	}

	b.mpMutext.Unlock()

}

func (b *simBrokerWorker) tickIsValid(tick *marketdata.Tick) bool {
	if !tick.HasQuote() && !tick.HasTrade() {
		return false
	}
	return true
}

func (b *simBrokerWorker) checkOrderExecutionOnTick(orderSim *simBrokerOrder, tick *marketdata.Tick) []event {

	err := b.validateOrderForExecution(orderSim, orderSim.Type)
	if err != nil {
		go b.newError(err)
		return nil
	}

	convertToList := func(e event) []event {
		if e == nil {
			return nil
		}
		return []event{e}
	}

	switch orderSim.Type {

	case MarketOrder:
		e := convertToList(b.checkOnTickMarket(orderSim, tick))
		return e
	case LimitOrder:
		e := convertToList(b.checkOnTickLimit(orderSim, tick))
		return e
	case StopOrder:
		e := convertToList(b.checkOnTickStop(orderSim, tick))
		return e
	case LimitOnClose:
		e := b.checkOnTickLOC(orderSim, tick)
		return e
	case LimitOnOpen:
		e := b.checkOnTickLOO(orderSim, tick)
		return e
	case MarketOnOpen:
		e := convertToList(b.checkOnTickMOO(orderSim, tick))
		return e
	case MarketOnClose:
		e := convertToList(b.checkOnTickMOC(orderSim, tick))
		return e
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

func (b *simBrokerWorker) checkOnTickLOO(order *simBrokerOrder, tick *marketdata.Tick) []event {
	err := b.validateOrderForExecution(order, LimitOnOpen)
	if err != nil {
		go b.newError(err)
		return nil
	}
	if !tick.IsOpening {
		if b.marketOpenUntilTime.Before(tick.Datetime) {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
			}

			return []event{&cancelE}
		}
		return nil
	}

	return b.checkOnTickLimitAuction(order, tick)

}

func (b *simBrokerWorker) checkOnTickLOC(order *simBrokerOrder, tick *marketdata.Tick) []event {
	err := b.validateOrderForExecution(order, LimitOnClose)
	if err != nil {
		go b.newError(err)
		return nil
	}

	if !tick.IsClosing {
		if b.marketCloseUntilTime.Before(tick.Datetime) {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			return []event{&cancelE}
		}
		return nil
	}

	return b.checkOnTickLimitAuction(order, tick)

}

func (b *simBrokerWorker) checkOnTickLimitAuction(order *simBrokerOrder, tick *marketdata.Tick) []event {
	var generatedEvents []event
	switch order.Side {
	case OrderSell:
		if tick.LastPrice < order.Price {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			generatedEvents = append(generatedEvents, &cancelE)
			return generatedEvents
		}

	case OrderBuy:
		if tick.LastPrice > order.Price {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
			}
			generatedEvents = append(generatedEvents, &cancelE)
			return generatedEvents
		}

	default:
		err := ErrUnknownOrderSide{
			OrdId:   order.Id,
			Message: "From checkOnTickLimitAuction",
			Caller:  "Sim Broker",
		}
		go b.newError(&err)
		return nil

	}

	if tick.LastPrice == order.Price && b.strictLimitOrders {
		cancelE := OrderCancelEvent{
			OrdId:     order.Id,
			BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), tick.Symbol),
		}
		generatedEvents = append(generatedEvents, &cancelE)
		return generatedEvents
	}

	execQty := order.Qty
	if execQty > tick.LastSize {
		execQty = tick.LastSize
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       execQty,
		BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
	}

	generatedEvents = append(generatedEvents, &fillE)

	if execQty < order.Qty {
		cancelE := OrderCancelEvent{
			OrdId:     order.Id,
			BaseEvent: be(tick.Datetime.Add(time.Duration(b.delay)*time.Millisecond), order.Symbol),
		}

		generatedEvents = append(generatedEvents, &cancelE)
	}

	if len(generatedEvents) == 0 {
		return nil
	}
	return generatedEvents

}

func (b *simBrokerWorker) checkOnTickMOO(order *simBrokerOrder, tick *marketdata.Tick) event {

	if !tick.IsOpening {
		return nil
	}

	if !tick.HasTrade() {
		return nil
	}

	err := b.validateOrderForExecution(order, MarketOnOpen)
	if err != nil {
		go b.newError(err)
		return nil
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       order.Qty,
		BaseEvent: be(tick.Datetime, order.Symbol),
	}
	return &fillE

}

func (b *simBrokerWorker) checkOnTickMOC(order *simBrokerOrder, tick *marketdata.Tick) event {
	//Todo подумать над реализацией когда отркрывающего тика вообще нет
	if !tick.IsClosing {
		return nil
	}

	if !tick.HasTrade() {
		return nil
	}

	err := b.validateOrderForExecution(order, MarketOnClose)
	if err != nil {
		go b.newError(err)
		return nil
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       order.Qty,
		BaseEvent: be(tick.Datetime, order.Symbol),
	}

	return &fillE

}

func (b *simBrokerWorker) checkOnTickLimit(order *simBrokerOrder, tick *marketdata.Tick) event {

	if !tick.HasTrade() {
		return nil
	}

	err := b.validateOrderForExecution(order, LimitOrder)
	if err != nil {
		go b.newError(err)
		return nil
	}

	lvsQty := order.Qty - order.BrokerExecQty

	switch order.Side {
	case OrderSell:
		if tick.LastPrice > order.Price {
			qty := lvsQty
			if tick.LastSize < int64(qty) {
				qty = tick.LastSize
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
					qty = tick.LastSize
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
				qty = tick.LastSize
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
					qty = tick.LastSize
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

func (b *simBrokerWorker) checkOnTickStop(order *simBrokerOrder, tick *marketdata.Tick) event {
	if !tick.HasTrade() {
		return nil
	}

	err := b.validateOrderForExecution(order, StopOrder)
	if err != nil {
		go b.newError(err)
		return nil
	}

	switch order.Side {
	case OrderSell:
		if tick.LastPrice > order.Price {
			return nil
		}
		price := tick.LastPrice
		lvsQty := order.Qty - order.BrokerExecQty
		qty := lvsQty
		if tick.LastSize < qty {
			qty = tick.LastSize
		}
		if tick.HasQuote() {
			price = tick.BidPrice
			qty = lvsQty
		}
		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     price,
			Qty:       qty,
			BaseEvent: be(tick.Datetime, order.Symbol),
		}

		return &fillE

	case OrderBuy:
		if tick.LastPrice < order.Price {
			return nil
		}
		price := tick.LastPrice
		lvsQty := order.Qty - order.BrokerExecQty
		qty := lvsQty
		if tick.LastSize < qty {
			qty = tick.LastSize
		}
		if tick.HasQuote() {
			price = tick.AskPrice
			qty = lvsQty
		}
		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     price,
			Qty:       qty,
			BaseEvent: be(tick.Datetime, order.Symbol),
		}

		return &fillE

	default:
		err := ErrUnknownOrderSide{
			OrdId:   order.Id,
			Message: "Got in checkOnTickStop",
			Caller:  "Sim Broker",
		}
		go b.newError(&err)
		return nil
	}

}

func (b *simBrokerWorker) checkOnTickMarket(order *simBrokerOrder, tick *marketdata.Tick) event {

	if order.Type != MarketOrder {
		go b.newError(&ErrUnexpectedOrderType{
			OrdId:        order.Id,
			Caller:       "SimBroker.checkOnTickMarket",
			ExpectedType: string(MarketOrder),
			ActualType:   string(order.Type),
		})

		return nil
	}

	if !order.isValid() {
		err := ErrInvalidOrder{
			OrdId:   order.Id,
			Message: "Can't check executions for invalid order.",
			Caller:  "simBrokerWorker",
		}
		go b.newError(&err)
		return nil
	}

	if tick.HasQuote() {
		var qty int64 = 0
		price := math.NaN()
		lvsQty := order.Qty - order.BrokerExecQty

		if order.Side == OrderBuy {
			if lvsQty > tick.AskSize {
				qty = tick.AskSize
			} else {
				qty = lvsQty
			}

			price = tick.AskPrice

		} else { //Short order logic + sanity check for Side issues
			if order.Side != OrderSell {
				go b.newError(errors.New("Sim Broker: unknown order side: " + string(order.Side)))
				return nil
			}

			if lvsQty > tick.BidSize {
				qty = tick.BidSize
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

		return &fillE

	} else { // If tick doesn't have quotes. Check on trades
		if !tick.HasTrade() {
			go b.newError(errors.New("Sim Broker: tick doesn't contain trade. "))
			return nil
		}

		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     tick.LastPrice,
			Qty:       order.Qty,
			BaseEvent: be(tick.Datetime, order.Symbol),
		}

		return &fillE
	}

}

//validateOrderForExecution checks if order is valid and can be filled. Returns nil if order is valid
//or error in other cases
func (b *simBrokerWorker) validateOrderForExecution(order *simBrokerOrder, expectedType OrderType) error {
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

	lvsQty := order.Qty - order.BrokerExecQty
	if lvsQty <= 0 {
		return errors.New("Sim broker: Lvs qty is zero or less. Nothing to execute. ")
	}

	return nil
}

func (b *simBrokerWorker) executeBrokerEvent(e event) {

	needWaitStrategyNotification := false

	switch i := e.(type) {

	case *OrderConfirmationEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			panic("Confirmation of not existing order")
		}
		ord.BrokerState = ConfirmedOrder
		ord.StateUpdTime = e.getTime()

	case *OrderCancelEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			msg := fmt.Sprintf("Can't find order %v in confirmed map to cancel it. ", i.OrdId)
			panic(msg)
		}

		ord.BrokerState = CanceledOrder
		ord.StateUpdTime = e.getTime()

	case *OrderFillEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			msg := fmt.Sprintf("Can't find order %v in confirmed map to fill it. ", i.OrdId)
			panic(msg)
		}
		needWaitStrategyNotification = true
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
		if ord.BrokerState != ConfirmedOrder {
			ord.BrokerState = RejectedOrder
		}

		ord.StateUpdTime = e.getTime()

	}
	if needWaitStrategyNotification {
		b.events <- e
	} else {
		go func() {
			b.events <- e
		}()
	}

}

func (b *simBrokerWorker) newError(e error) {
	if b.errChan == nil {
		panic("Simulated broker error chan is nil")
	}
	b.errChan <- e
}
