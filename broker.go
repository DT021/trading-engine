package engine

import (
	"alex/marketdata"
	"errors"
	"time"
)

type IBroker interface {
	Connect(errChan chan error, eventChan chan *event) error
	OnNewOrder(e *NewOrderEvent)
	IsSimulated() bool
	OnCandleClose(candle *marketdata.Candle)
	OnCandleOpen(price float64)
	OnTick(candle *marketdata.Tick)
	NextEvent()
	PopEvent()
}

type SimulatedBroker struct {
	errChan         chan error
	eventChan       chan *event
	filledOrders    map[string]*Order
	canceledOrders  map[string]*Order
	confirmedOrders map[string]*Order
	rejectedOrders  map[string]*Order
	allOrders       map[string]*Order
	delay           int64
}

func (b *SimulatedBroker) Connect(errChan chan error, eventChan chan *event) error {
	if errChan == nil {
		return errors.New("Can't connect simulated broker. Error chan is nil. ")
	}
	if eventChan == nil {
		return errors.New("Can't connect simulated broker. Event chan is nil. ")
	}
	b.errChan = errChan
	b.eventChan = eventChan

	b.filledOrders = make(map[string]*Order)
	b.canceledOrders = make(map[string]*Order)
	b.confirmedOrders = make(map[string]*Order)
	b.rejectedOrders = make(map[string]*Order)
	b.allOrders = make(map[string]*Order)

	return nil
}

func (b *SimulatedBroker) OnNewOrder(e *NewOrderEvent) {

	if !e.LinkedOrder.isValid() {
		r := "Sim Broker: can't confirm order. Order is not valid"
		rejectEvent := OrderRejectedEvent{OrdId: e.LinkedOrder.Id, Reason: r, Time: time.Now()}
		go b.newEvent(&rejectEvent)
		b.rejectedOrders[e.LinkedOrder.Id] = e.LinkedOrder
		b.allOrders[e.LinkedOrder.Id] = e.LinkedOrder
		return
	}

	if _, ok := b.allOrders[e.LinkedOrder.Id]; ok {
		r := "Sim Broker: can't confirm order. Order with this ID already exists on broker side"
		rejectEvent := OrderRejectedEvent{OrdId: e.LinkedOrder.Id, Reason: r, Time: time.Now()}
		go b.newEvent(&rejectEvent)
		b.rejectedOrders[e.LinkedOrder.Id] = e.LinkedOrder

		return
	}

	b.allOrders[e.LinkedOrder.Id] = e.LinkedOrder
	confEvent := OrderConfirmationEvent{e.LinkedOrder.Id, time.Now()}

	go b.newEvent(&confEvent)

	b.confirmedOrders[e.LinkedOrder.Id] = e.LinkedOrder
}

func (b *SimulatedBroker) newEvent(e event) {
	if b.eventChan == nil {
		panic("Simulated broker event chan is nil")
	}
	time.Sleep(time.Duration(b.delay) * time.Millisecond)
	b.eventChan <- &e

}

func (b *SimulatedBroker) newError(e error) {
	if b.errChan == nil {
		panic("Simulated broker error chan is nil")
	}
	b.errChan <- e
}
