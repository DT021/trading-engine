package engine

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"time"
	"math"
)

func newTestSimulatedBroker() *SimulatedBroker {
	b := SimulatedBroker{delay:100}
	errChan := make(chan error)
	eventChan := make(chan *event)
	err := b.Connect(errChan, eventChan)
	if err != nil {
		panic(err)
	}
	return &b
}

func TestSimulatedBroker_Connect(t *testing.T) {
	t.Log("Test connect simulated broker")
	{

		b := SimulatedBroker{}
		errChan := make(chan error)
		eventChan := make(chan *event)
		err := b.Connect(errChan, eventChan)

		assert.Nil(t, err)

		order := newTestOrder(10, OrderSell, 10, "15")
		b.filledOrders[order.Id] = order
		b.canceledOrders[order.Id] = order
		b.rejectedOrders[order.Id] = order
		b.confirmedOrders[order.Id] = order
		b.allOrders[order.Id] = order

		assert.NotNil(t, b.errChan)
		assert.NotNil(t, b.eventChan)
	}

	t.Log("Test in-build test simulated broker creator")
	{
		b := newTestSimulatedBroker()
		order := newTestOrder(10, OrderSell, 10, "15")
		b.filledOrders[order.Id] = order
		b.canceledOrders[order.Id] = order
		b.rejectedOrders[order.Id] = order
		b.confirmedOrders[order.Id] = order
		b.allOrders[order.Id] = order

		assert.NotNil(t, b.errChan)
		assert.NotNil(t, b.eventChan)

	}

}

func TestSimulatedBroker_OnNewOrder(t *testing.T) {
	b := newTestSimulatedBroker()
	eventChan := b.eventChan
	t.Log("Sim broker: test normal new order")
	{
		order := newTestOrder(10.1, OrderSell, 100, "id1")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order, Time: time.Now()})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 1)

		v := <-eventChan
		switch (*v).(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", (*v).getName())
		}

	}

	t.Log("Sim broker: test new order with duplicate ID")
	{
		order := newTestOrder(10.1, OrderSell, 100, "id1")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order, Time: time.Now()})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 1)
		assert.Len(t, b.allOrders, 1)

		v := <-eventChan
		switch (*v).(type) {
		case *OrderRejectedEvent:
			assert.Equal(t, "Sim Broker: can't confirm order. Order with this ID already exists on broker side", (*v).(*OrderRejectedEvent).Reason)
			t.Log("OK! Got OrderRejectedEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderRejectedEvent. Got %v", (*v).getName())
		}

	}

	t.Log("Sim broker: test new order with wrong params")
	{
		order := newTestOrder(math.NaN(), OrderSell, 100, "id1")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order, Time: time.Now()})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 1)
		assert.Len(t, b.allOrders, 1)

		v := <-eventChan
		switch (*v).(type) {
		case *OrderRejectedEvent:
			assert.Equal(t, "Sim Broker: can't confirm order. Order is not valid", (*v).(*OrderRejectedEvent).Reason)
			t.Log("OK! Got OrderRejectedEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderRejectedEvent. Got %v", (*v).getName())
		}

	}

}
