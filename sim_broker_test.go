package engine

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"time"
	"math"
	"alex/marketdata"
)

func newTestSimulatedBroker() *SimulatedBroker {
	b := SimulatedBroker{delay: 100}
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

func TestSimulatedBroker_OnCancelRequest(t *testing.T) {
	b := newTestSimulatedBroker()

	t.Log("Sim Broker: normal cancel request")
	{
		order := newTestOrder(15, OrderSell, 100, "1")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 1)

		v := <-b.eventChan
		switch (*v).(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", (*v).getName())
		}

		order.State = ConfirmedOrder // Mock it

		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: order.Id})

		assert.Len(t, b.confirmedOrders, 0)
		assert.Len(t, b.canceledOrders, 1)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 1)

		v = <-b.eventChan
		switch (*v).(type) {
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderCancelEvent. Got %v", (*v).getName())
		}

	}

	t.Log("Sim Broker: cancel already canceled order")
	{
		ordId := ""
		for k, _ := range b.canceledOrders {
			ordId = k
		}

		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: ordId})
		v := <-b.errChan
		assert.Error(t, v, "Sim broker: Can't cancel order. ID not found in confirmed. ")
	}

	t.Log("Sim broker: cancel not existing order")
	{
		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: "Not existing ID"})
		v := <-b.errChan
		assert.Error(t, v, "Sim broker: Can't cancel order. ID not found in confirmed. ")
	}

	t.Log("Sim broker: cancel order with not confirmed status")
	{
		order := newTestOrder(15, OrderSell, 100, "id2")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order, Time: time.Now()})

		v := <-b.eventChan
		switch (*v).(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", (*v).getName())
		}

		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: "id2"})
		err := <-b.errChan
		assert.Error(t, err, "Sim broker: Can't cancel order. Order state is not ConfirmedOrder ")

	}
}

func assertNoEventsGeneratedByBroker(t *testing.T, b *SimulatedBroker) {
	select {
	case v, ok := <-b.eventChan:
		assert.False(t, ok)
		if ok {
			t.Errorf("ERROR! Expected no events. Found: %v", v)
		}
	default:
		t.Log("OK! Events chan is empty")
		break
	}
}

func assertNoErrorsGeneratedByBroker(t *testing.T, b *SimulatedBroker) {
	select {
	case v, ok := <-b.errChan:
		assert.False(t, ok)
		if ok {
			t.Errorf("ERROR! Expected no errors. Found: %v", v)
		}
	default:
		t.Log("OK! Error chan is empty")
		break
	}
}

func getTestSimBrokerGeneratedEvent(t *testing.T, b *SimulatedBroker) *event {
	//time.Sleep(time.Duration(b.delay*2) * time.Millisecond)
	select {
	case v := <-b.eventChan:
		return v
		/*default:
			return nil*/
	}
	return nil
}

func TestSimulatedBroker_checkOnTickMarket(t *testing.T) {
	b := newTestSimulatedBroker()

	t.Log("Case when sim broker has only trades feed")
	{
		b.hasQuotesAndTrades = false

		t.Log("Sim broker: test normal market order execution on tick")
		{
			t.Log("Check execution when we have only last trade price")
			{

				order := newTestOrder(math.NaN(), OrderSell, 100, "Market1")
				order.Type = MarketOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickMarket(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)
				//assertNoEventsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.01, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

			t.Log("Check execution when we get unexpected tick with quotes")
			{
				order := newTestOrder(math.NaN(), OrderSell, 100, "Market2")
				order.Type = MarketOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.00,
					AskPrice:  20.05,
					BidSize:   200,
					AskSize:   300,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickMarket(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				v := <-b.errChan

				assert.NotNil(t, v)
			}

		}
	}

	t.Log("Case when sim broker has both quotes and trades feed. ")
	{
		b.hasQuotesAndTrades = true
		t.Log("Sim broker: normal execution on tick with quotes")
		{
			t.Log("SHORT ORDER full execution")
			{

				order := newTestOrder(math.NaN(), OrderSell, 100, "Market3")
				order.Type = MarketOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.95,
					AskPrice:  20.05,
					BidSize:   200,
					AskSize:   300,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickMarket(order, &tick)
				assert.Len(t, b.confirmedOrders, 1) //Prev order wasn't executed

				assert.Equal(t, ConfirmedOrder, order.State)
				assertNoErrorsGeneratedByBroker(t, b)
				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 19.95, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

			t.Log("LONG ORDER full execution")
			{

				order := newTestOrder(math.NaN(), OrderBuy, 100, "Market4")
				order.Type = MarketOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.95,
					AskPrice:  21.05,
					BidSize:   200,
					AskSize:   300,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.Len(t, b.confirmedOrders, 1)
				assert.Equal(t, ConfirmedOrder, order.State)
				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 21.05, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

			t.Log("SHORT ORDER partial execution")
			{

				order := newTestOrder(math.NaN(), OrderSell, 500, "Market5")
				order.Type = MarketOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.01,
					AskPrice:  20.05,
					BidSize:   200,
					AskSize:   300,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.Len(t, b.confirmedOrders, 2)
				assert.Equal(t, ConfirmedOrder, order.State)
				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.01, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}

				//New tick - fill rest of the order. New price*************************
				order.ExecQty = 200
				order.State = PartialFilledOrder

				tick = marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.98,
					AskPrice:  20.05,
					BidSize:   800,
					AskSize:   300,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.Len(t, b.confirmedOrders, 1) //Order goes to filled

				v = getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 19.98, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 300, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

			t.Log("LONG ORDER partial execution")
			{

				order := newTestOrder(math.NaN(), OrderBuy, 900, "Market6")
				order.Type = MarketOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.01,
					AskPrice:  20.09,
					BidSize:   200,
					AskSize:   600,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.Len(t, b.confirmedOrders, 2)
				assert.Equal(t, ConfirmedOrder, order.State)
				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.09, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 600, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}

				//New tick - fill rest of the order. New price*************************
				order.ExecQty = 600
				order.State = PartialFilledOrder

				tick = marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.98,
					AskPrice:  20.05,
					BidSize:   800,
					AskSize:   300,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.Len(t, b.confirmedOrders, 1) //Order goes to filled

				v = getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.05, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 300, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

		}
		t.Log("Sim broker: skip execution when there are no quotes on tick")
		{
			order := newTestOrder(math.NaN(), OrderSell, 100, "Market7")
			order.Type = MarketOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				BidSize:   0,
				AskSize:   0,
				HasTrade:  true,
			}

			b.checkOnTickMarket(order, &tick)
			//Expect no events and errors
			assertNoErrorsGeneratedByBroker(t, b)
			assertNoEventsGeneratedByBroker(t, b)

		}

		t.Log("Sim broker: put error in chan when order is not valid")
		{
			order := newTestOrder(20.0, OrderSell, 100, "Market7")
			order.Type = MarketOrder
			order.State = ConfirmedOrder
			assert.False(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  19.95,
				AskPrice:  20.01,
				BidSize:   100,
				AskSize:   200,
				HasTrade:  true,
			}

			b.checkOnTickMarket(order, &tick)
			err := <-b.errChan
			assert.NotNil(t, err)
			assertNoEventsGeneratedByBroker(t, b)
		}

		t.Log("Sim broker: put error in chan when order is not in Confirmed or PartialFilledState")
		{
			order := newTestOrder(20.0, OrderSell, 100, "Market7")
			order.Type = MarketOrder
			order.State = NewOrder
			assert.False(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  19.95,
				AskPrice:  20.01,
				BidSize:   100,
				AskSize:   200,
				HasTrade:  true,
			}

			b.checkOnTickMarket(order, &tick)
			err := <-b.errChan
			assert.NotNil(t, err)
			assertNoEventsGeneratedByBroker(t, b)
		}
	}
}

func TestSimulatedBroker_execs(t *testing.T) {

	t.Log("Sim broker: test limit order execution on tick")
	{

	}

	t.Log("Sim broker: test stop order execution on tick")
	{

	}

	t.Log("Sim broker: test market on open order execution on tick")
	{

	}

	t.Log("Sim broker: test market on close order execution on tick")
	{

	}

	t.Log("Sim broker: test limit on close order execution on tick")
	{

	}

	t.Log("Sim broker: test limit on open order execution on tick")
	{

	}
}
