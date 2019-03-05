package engine

import (
	"alex/marketdata"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type eventArray []event

func (t eventArray) sort() {
	sort.SliceStable(t, func(i, j int) bool {
		return t[i].getTime().Unix() < t[j].getTime().Unix()
	})
}

type IBroker interface {
	Connect()
	Disconnect()
	Init(errChan chan error, events chan event, symbols []string)
	IsSimulated() bool
	Notify(e event)
	shutDown()
}

type simBrokerOrder struct {
	*Order
	BrokerState   OrderState
	StateUpdTime  time.Time
	BrokerExecQty int64
	BrokerPrice   float64
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
			delay:                b.delay,
			strictLimitOrders:    b.strictLimitOrders,
			marketOpenUntilTime:  b.marketCloseUntilTime,
			marketCloseUntilTime: b.marketCloseUntilTime,
			mpMutext:             &sync.RWMutex{},
			waitGroup:            &sync.WaitGroup{},
			orders:               make(map[string]*simBrokerOrder),
		}
		b.workers[s] = &bw

	}
}

func (b *SimBroker) Notify(e event) {
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

func (b *SimBroker) shutDown(){
	for _, w := range b.workers{
		w.shutDown()
	}
}

func (b *SimBroker) IsSimulated() bool {
	return true
}

//*******simBrokerWorker**********************************************
type simBrokerWorker struct {
	symbol               string
	errChan              chan error
	events               chan event
	delay                int64
	strictLimitOrders    bool
	marketOpenUntilTime  TimeOfDay
	marketCloseUntilTime TimeOfDay
	mpMutext             *sync.RWMutex
	orders               map[string]*simBrokerOrder
	generatedEvents      eventArray
	waitGroup            *sync.WaitGroup
}

func (b *simBrokerWorker) onNewOrder(e *NewOrderEvent) {
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()

	if !e.LinkedOrder.isValid() {
		r := "Sim Broker: can't confirm order. Order is not valid"
		rejectEvent := OrderRejectedEvent{
			OrdId:     e.LinkedOrder.Id,
			Reason:    r,
			BaseEvent: be(b.genEventTime(e.getTime()), e.Symbol),
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
			BaseEvent: be(b.genEventTime(e.getTime()), e.Symbol),
		}
		b.executeBrokerEvent(&rejectEvent)

		return
	}

	confEvent := OrderConfirmationEvent{
		OrdId:     e.LinkedOrder.Id,
		BaseEvent: be(b.genEventTime(e.getTime()), e.Symbol),
	}

	b.orders[e.LinkedOrder.Id] = &simBrokerOrder{
		Order:         e.LinkedOrder,
		BrokerState:   NewOrder,
		BrokerExecQty: 0,
		BrokerPrice:   e.LinkedOrder.Price,
		StateUpdTime:  confEvent.getTime(),
	}

	b.executeBrokerEvent(&confEvent)

}

func (b *simBrokerWorker) onCancelRequest(e *OrderCancelRequestEvent) {
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()
	newEvTime := b.genEventTime(e.getTime())

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

	newEvTime := b.genEventTime(e.getTime())

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

		b.newError(&err)
		return

	}
	b.mpMutext.Lock()
	defer b.mpMutext.Unlock()

	if len(b.orders) == 0 && len(b.generatedEvents) == 0 {
		return
	}

	var genEvents []event

	for _, o := range b.orders {
		if o.Symbol == tick.Symbol && (o.BrokerState == ConfirmedOrder || o.BrokerState == PartialFilledOrder) {
			if o.StateUpdTime.Before(tick.Datetime) {
				e := b.checkOrderExecutionOnTick(o, tick)
				if e != nil {
					genEvents = append(genEvents, e...)
				}
			}
		}
	}

	if len(genEvents) > 0 {
		for _, e := range genEvents {
			b.executeBrokerEvent(e)
		}
	}

	if len(b.generatedEvents) == 0 {
		return
	}

	b.generatedEvents.sort()

	var eventsLeft eventArray
	var eventsToSend eventArray

	for _, e := range b.generatedEvents {
		if !e.getTime().After(tick.Datetime) {
			eventsToSend = append(eventsToSend, e)
		} else {
			eventsLeft = append(eventsLeft, e)
		}
	}

	b.generatedEvents = eventsLeft

	for _, e := range eventsToSend {
		b.events <- e
	}

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
		b.newError(err)
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
		b.newError(&err)
	}

	return nil

}

func (b *simBrokerWorker) checkOnTickLOO(order *simBrokerOrder, tick *marketdata.Tick) []event {
	err := b.validateOrderForExecution(order, LimitOnOpen)
	if err != nil {
		b.newError(err)
		return nil
	}
	if !tick.IsOpening {
		if b.marketOpenUntilTime.Before(tick.Datetime) {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
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
		b.newError(err)
		return nil
	}

	if !tick.IsClosing {
		if b.marketCloseUntilTime.Before(tick.Datetime) {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(b.genEventTime(tick.Datetime), tick.Symbol),
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
		if tick.LastPrice < order.BrokerPrice {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(b.genEventTime(tick.Datetime), tick.Symbol),
			}
			generatedEvents = append(generatedEvents, &cancelE)
			return generatedEvents
		}

	case OrderBuy:
		if tick.LastPrice > order.BrokerPrice {
			cancelE := OrderCancelEvent{
				OrdId:     order.Id,
				BaseEvent: be(b.genEventTime(tick.Datetime), tick.Symbol),
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
		b.newError(&err)
		return nil

	}

	if tick.LastPrice == order.BrokerPrice && b.strictLimitOrders {
		cancelE := OrderCancelEvent{
			OrdId:     order.Id,
			BaseEvent: be(b.genEventTime(tick.Datetime), tick.Symbol),
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
		BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
	}

	generatedEvents = append(generatedEvents, &fillE)

	if execQty < order.Qty {
		cancelE := OrderCancelEvent{
			OrdId:     order.Id,
			BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
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
		b.newError(err)
		return nil
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       order.Qty,
		BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
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
		b.newError(err)
		return nil
	}

	fillE := OrderFillEvent{
		OrdId:     order.Id,
		Price:     tick.LastPrice,
		Qty:       order.Qty,
		BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
	}

	return &fillE

}

func (b *simBrokerWorker) checkOnTickLimit(order *simBrokerOrder, tick *marketdata.Tick) event {

	if !tick.HasTrade() {
		return nil
	}

	err := b.validateOrderForExecution(order, LimitOrder)
	if err != nil {
		b.newError(err)
		return nil
	}

	lvsQty := order.Qty - order.BrokerExecQty

	switch order.Side {
	case OrderSell:
		if tick.LastPrice > order.BrokerPrice {
			qty := lvsQty
			if tick.LastSize < int64(qty) {
				qty = tick.LastSize
			}

			fillE := OrderFillEvent{
				OrdId:     order.Id,
				Price:     order.BrokerPrice,
				Qty:       qty,
				BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
			}
			return &fillE

		} else {
			if tick.LastPrice == order.BrokerPrice && !b.strictLimitOrders {
				qty := lvsQty
				if tick.LastSize < int64(qty) {
					qty = tick.LastSize
				}

				fillE := OrderFillEvent{
					OrdId:     order.Id,
					Price:     order.BrokerPrice,
					Qty:       qty,
					BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
				}

				return &fillE
			} else {
				return nil
			}
		}

	case OrderBuy:
		if tick.LastPrice < order.BrokerPrice {
			qty := lvsQty
			if tick.LastSize < int64(qty) {
				qty = tick.LastSize
			}

			fillE := OrderFillEvent{
				OrdId:     order.Id,
				Price:     order.BrokerPrice,
				Qty:       qty,
				BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
			}

			return &fillE

		} else {
			if tick.LastPrice == order.BrokerPrice && !b.strictLimitOrders {
				qty := lvsQty
				if tick.LastSize < int64(qty) {
					qty = tick.LastSize
				}

				fillE := OrderFillEvent{
					OrdId:     order.Id,
					Price:     order.BrokerPrice,
					Qty:       qty,
					BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
				}

				return &fillE
			} else {
				return nil
			}
		}
	default:
		b.newError(errors.New("Sim broker: can't check fill for order. Unknown side. "))
		return nil

	}

}

func (b *simBrokerWorker) genEventTime(baseTime time.Time) time.Time {
	newEvTime := baseTime.Add(time.Duration(b.delay) * time.Millisecond)
	return newEvTime
}

func (b *simBrokerWorker) checkOnTickStop(order *simBrokerOrder, tick *marketdata.Tick) event {
	if !tick.HasTrade() {
		return nil
	}

	err := b.validateOrderForExecution(order, StopOrder)
	if err != nil {
		b.newError(err)
		return nil
	}

	switch order.Side {
	case OrderSell:
		if tick.LastPrice > order.BrokerPrice {
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
			BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
		}

		return &fillE

	case OrderBuy:
		if tick.LastPrice < order.BrokerPrice {
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
			BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
		}

		return &fillE

	default:
		err := ErrUnknownOrderSide{
			OrdId:   order.Id,
			Message: "Got in checkOnTickStop",
			Caller:  "Sim Broker",
		}
		b.newError(&err)
		return nil
	}

}

func (b *simBrokerWorker) checkOnTickMarket(order *simBrokerOrder, tick *marketdata.Tick) event {

	if order.Type != MarketOrder {
		b.newError(&ErrUnexpectedOrderType{
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
		b.newError(&err)
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
				b.newError(errors.New("Sim Broker: unknown order side: " + string(order.Side)))
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
			BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
		}

		return &fillE

	} else { // If tick doesn't have quotes. Check on trades
		if !tick.HasTrade() {
			b.newError(errors.New("Sim Broker: tick doesn't contain trade. "))
			return nil
		}

		fillE := OrderFillEvent{
			OrdId:     order.Id,
			Price:     tick.LastPrice,
			Qty:       order.Qty,
			BaseEvent: be(b.genEventTime(tick.Datetime), order.Symbol),
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
			Message: fmt.Sprintf("Order : %+v", order),
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

	//TODO нет реджектов по канселу и реплейсу

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

	case *OrderReplacedEvent:
		ord, ok := b.orders[i.OrdId]
		if !ok {
			msg := fmt.Sprintf("Can't find order %v in confirmed map to replace it. ", i.OrdId)
			panic(msg)
		}

		ord.StateUpdTime = e.getTime()
		ord.BrokerPrice = i.NewPrice

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
		if ord.BrokerState != ConfirmedOrder {
			ord.BrokerState = RejectedOrder
		}

		ord.StateUpdTime = e.getTime()

	}
	b.generatedEvents = append(b.generatedEvents, e)

}

func (b *simBrokerWorker) newError(e error) {
	if b.errChan == nil {
		panic("Simulated broker error chan is nil")
	}
	b.waitGroup.Add(1)
	go func() {
		b.errChan <- e
		b.waitGroup.Done()
	}()

}

func (b *simBrokerWorker) shutDown(){
	b.waitGroup.Wait()
}
