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
	Init(err chan error, symbols []string)
	SubscribeEvents()
	UnSubscribeEvents()
	SetSymbolChannels(symbol string, bs BrokerSymbolChannels)
	IsSimulated() bool
}

type BrokerSymbolChannels struct {
	signals        chan event
	broker         chan event
	brokerNotifier chan struct{}
	notifyBroker   chan *BrokerNotifyEvent
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

func (b *SimBroker) Connect() {
	fmt.Println("SimBroker connected")
}

func (b *SimBroker) Init(errChan chan error, symbols []string) {
	if len(symbols) == 0 {
		panic("No symbols specified")
	}

	b.workers = make(map[string]*SimulatedBrokerWorker)

	for _, s := range symbols {
		bw := SimulatedBrokerWorker{
			symbol:               s,
			errChan:              errChan,
			terminationChan:      make(chan struct{}),
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

func (b *SimBroker) SubscribeEvents() {
	for _, w := range b.workers {
		go w.run()
	}

}

func (b *SimBroker) UnSubscribeEvents() {
	wg := &sync.WaitGroup{}
	for _, w := range b.workers {
		go func() {
			wg.Add(1)
			w.terminationChan <- struct{}{}
			wg.Done()
		}()
	}
	wg.Wait()

}

func (b *SimBroker) SetSymbolChannels(symbol string, bs BrokerSymbolChannels) {
	w, ok := b.workers[symbol]
	if !ok {
		panic("Can't set channel for symbol not in workers")
	}

	w.ch = bs

}

func (b *SimBroker) IsSimulated() bool {
	return true
}

type SimulatedBrokerWorker struct {
	symbol string

	errChan         chan error
	ch              BrokerSymbolChannels
	terminationChan chan struct{}

	delay                int64
	hasQuotesAndTrades   bool
	strictLimitOrders    bool
	marketOpenUntilTime  TimeOfDay
	marketCloseUntilTime TimeOfDay

	mpMutext *sync.RWMutex
	orders   map[string]*SimBrokerOrder
}

func (b *SimulatedBrokerWorker) proxyEvent(e event) {
	switch i := e.(type) {
	case *NewOrderEvent:
		b.onNewOrder(i)
	case *OrderCancelRequestEvent:
		b.onCancelRequest(i)
	case *OrderReplaceRequestEvent:
		b.onReplaceRequest(i)
	case *NewTickEvent:
		b.onTick(i.Tick)
	default:
		panic("Unexpected event time in broker: " + e.getName())

	}

}

func (b *SimulatedBrokerWorker) shutDown() {

}

func (b *SimulatedBrokerWorker) run() {
Loop:
	for {
		select {
		case e := <-b.ch.signals:
			b.proxyEvent(e)
		case <-b.terminationChan:
			break Loop
		}
	}
	b.shutDown()

}

func (b *SimulatedBrokerWorker) onNewOrder(e *NewOrderEvent) {
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
	fmt.Println("Order confirmed")

}

func (b *SimulatedBrokerWorker) onCancelRequest(e *OrderCancelRequestEvent) {
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

func (b *SimulatedBrokerWorker) onReplaceRequest(e *OrderReplaceRequestEvent) {
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

	b.newEvent(&replacedEvent)

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

func (b *SimulatedBrokerWorker) onTick(tick *marketdata.Tick) {
	defer func() {
		b.ch.brokerNotifier <- struct{}{}
	}()

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

func (b *SimulatedBrokerWorker) checkOrderExecutionOnTick(orderSim *SimBrokerOrder, tick *marketdata.Tick) []event {

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

func (b *SimulatedBrokerWorker) checkOnTickLOO(order *SimBrokerOrder, tick *marketdata.Tick) []event {

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

func (b *SimulatedBrokerWorker) checkOnTickLOC(order *SimBrokerOrder, tick *marketdata.Tick) []event {
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

func (b *SimulatedBrokerWorker) checkOnTickLimitAuction(order *SimBrokerOrder, tick *marketdata.Tick) []event {
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
	if execQty > int(tick.LastSize) {
		execQty = int(tick.LastSize)
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

func (b *SimulatedBrokerWorker) checkOnTickMOO(order *SimBrokerOrder, tick *marketdata.Tick) event {

	if !tick.IsOpening {
		return nil
	}

	if !tick.HasTrade {
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

func (b *SimulatedBrokerWorker) checkOnTickMOC(order *SimBrokerOrder, tick *marketdata.Tick) event {
	//Todo подумать над реализацией когда отркрывающего тика вообще нет
	if !tick.IsClosing {
		return nil
	}

	if !tick.HasTrade {
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

func (b *SimulatedBrokerWorker) checkOnTickStop(order *SimBrokerOrder, tick *marketdata.Tick) event {
	if !tick.HasTrade {
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

		return &fillE

	case OrderBuy:
		if tick.LastPrice < order.Price {
			return nil
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

func (b *SimulatedBrokerWorker) checkOnTickMarket(order *SimBrokerOrder, tick *marketdata.Tick) event {

	if b.hasQuotesAndTrades && !tick.HasQuote {
		return nil
	}
	if !b.hasQuotesAndTrades && tick.HasQuote {
		go b.newError(errors.New("Sim Broker: broker doesn't expect quotes. Only trades. "))
		return nil
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
				return nil
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

		return &fillE

	} else { //If broker accepts only trades without quotes
		if !tick.HasTrade {
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
	if b.ch.broker == nil {
		panic("Simulated broker event chan is nil")
	}

	waitForNotification := false

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
		waitForNotification = true
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

	b.ch.broker <- e
	if waitForNotification {
		fmt.Println("Waiting for notification")
		<-b.ch.notifyBroker
		fmt.Println("Got notification")
	}

}

func (b *SimulatedBrokerWorker) newError(e error) {
	if b.errChan == nil {
		panic("Simulated broker error chan is nil")
	}
	b.errChan <- e
}
