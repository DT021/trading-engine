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

func getTestSimBrokerGeneratedErrors(t *testing.T, b *SimulatedBroker) error {

	select {
	case v := <-b.errChan:
		return v

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

		//Todo add test when we put not market order. should be error
	}
}

func TestSimulatedBroker_checkOnTickLimit(t *testing.T) {
	b := newTestSimulatedBroker()

	t.Log("Sim broker: test tick and order on different symbols")
	{
		//todo
	}

	t.Log("Sim broker: test unknow order side")
	{
		//todo
	}

	t.Log("Sim broker: test limit exectution for not limit order")
	{
		order := newTestOrder(math.NaN(), OrderBuy, 200, "Wid1")
		order.Type = MarketOrder

		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.confirmedOrders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
		}

		b.checkOnTickLimit(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)
		assertNoEventsGeneratedByBroker(t, b)
		err := getTestSimBrokerGeneratedErrors(t, b)

		assert.NotNil(t, err) //Todo сделать проверку на конкретные ошибки

		delete(b.confirmedOrders, order.Id)

	}

	t.Log("Sim broker: test limit execution for already filled order")
	{
		order := newTestOrder(20.01, OrderBuy, 200, "Wid10")

		order.State = FilledOrder
		order.ExecQty = 200
		assert.True(t, order.isValid())
		b.filledOrders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
		}

		b.checkOnTickLimit(order, &tick)
		assert.Equal(t, FilledOrder, order.State)
		assert.Len(t, b.filledOrders, 1)
		assertNoEventsGeneratedByBroker(t, b)
		err := getTestSimBrokerGeneratedErrors(t, b)

		assert.NotNil(t, err) //Todo сделать проверку на конкретные ошибки

		delete(b.filledOrders, order.Id)
	}

	t.Log("Sim broker: test limit execution for not valid order")
	{
		order := newTestOrder(math.NaN(), OrderBuy, 200, "Wid2")

		assert.False(t, order.isValid())

		order.State = ConfirmedOrder

		b.confirmedOrders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
		}

		b.checkOnTickLimit(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)
		assertNoEventsGeneratedByBroker(t, b)
		err := getTestSimBrokerGeneratedErrors(t, b) //Todo сделать проверку на конкретные ошибки

		assert.NotNil(t, err)

		delete(b.confirmedOrders, order.Id)
	}

	t.Log("Sim broker: test limit execution for tick without trade")
	{
		order := newTestOrder(20, OrderBuy, 200, "Wid3")

		assert.True(t, order.isValid())

		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.confirmedOrders[order.Id] = order

		//Case when tick has tag but don't have price
		tick := marketdata.Tick{
			LastPrice: math.NaN(),
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
		}

		b.checkOnTickLimit(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)
		assertNoEventsGeneratedByBroker(t, b)
		assertNoErrorsGeneratedByBroker(t, b)

		//Case when tick has right Tag
		tick = marketdata.Tick{
			LastPrice: math.NaN(),
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  false,
			HasQuote:  false,
		}

		b.checkOnTickLimit(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)
		assertNoEventsGeneratedByBroker(t, b)
		assertNoErrorsGeneratedByBroker(t, b)

		delete(b.confirmedOrders, order.Id)
	}

	t.Log("Sim broker: test limit execution for not confirmed order")
	{
		order := newTestOrder(20, OrderBuy, 200, "Wid4")

		assert.True(t, order.isValid())

		order.State = NewOrder
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 19.88,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
		}

		b.checkOnTickLimit(order, &tick)
		assert.Equal(t, NewOrder, order.State)

		assertNoEventsGeneratedByBroker(t, b)
		err := getTestSimBrokerGeneratedErrors(t, b) //Todo сделать проверку на конкретные ошибки

		assert.NotNil(t, err)
	}

	t.Log("Sim broker: TEST LONG ORDERS")
	{

		t.Log("Sim broker: test limit order execution on tick")
		{
			t.Log("Normal execution")
			{
				order := newTestOrder(20.02, OrderBuy, 200, "id1")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.00,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}

			}

			t.Log("Partial order fill")
			{
				order := newTestOrder(20.13, OrderBuy, 200, "id2")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				t.Log("First tick without execution")
				{
					tick := marketdata.Tick{
						LastPrice: 20.15,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
						HasTrade:  true,
						HasQuote:  false,
					}

					b.checkOnTickLimit(order, &tick)
					assert.Equal(t, ConfirmedOrder, order.State)
					assert.Len(t, b.confirmedOrders, 1)
					assertNoErrorsGeneratedByBroker(t, b)
					assertNoEventsGeneratedByBroker(t, b)
				}

				t.Log("First fill")
				{
					tick := marketdata.Tick{
						LastPrice: 20.12,
						LastSize:  100,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
						HasTrade:  true,
						HasQuote:  false,
					}

					b.checkOnTickLimit(order, &tick)
					assert.Equal(t, ConfirmedOrder, order.State)
					assert.Len(t, b.confirmedOrders, 1)
					assertNoErrorsGeneratedByBroker(t, b)

					v := getTestSimBrokerGeneratedEvent(t, b)
					assert.NotNil(t, v)
					order.ExecQty = 100

					switch (*v).(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
						assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
					}

				}

				t.Log("Complete fill")
				{
					tick := marketdata.Tick{
						LastPrice: 20.12,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
						HasTrade:  true,
						HasQuote:  false,
					}

					b.checkOnTickLimit(order, &tick)
					assert.Equal(t, ConfirmedOrder, order.State)
					assert.Len(t, b.confirmedOrders, 0)
					assertNoErrorsGeneratedByBroker(t, b)

					v := getTestSimBrokerGeneratedEvent(t, b)
					assert.NotNil(t, v)
					order.ExecQty = 200

					switch (*v).(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
						assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
					}

				}

			}

		}

		t.Log("Sim broker: test strict Limit ordres")
		{
			b.strictLimitOrders = true

			order := newTestOrder(10.02, OrderBuy, 200, "id3")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			t.Log("Tick with order price but without fill")
			{

				tick := marketdata.Tick{
					LastPrice: 10.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoErrorsGeneratedByBroker(t, b)
				assertNoEventsGeneratedByBroker(t, b)
			}

			t.Log("Tick below order price")
			{
				tick := marketdata.Tick{
					LastPrice: 10.01,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

		}

		t.Log("Sim broker: test not strict Limit orders")
		{
			b.strictLimitOrders = false

			t.Log("Tick with order price with fill")
			{
				order := newTestOrder(10.15, OrderBuy, 200, "id4")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  500,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}

			}

			t.Log("Tick below order price")
			{
				order := newTestOrder(10.08, OrderBuy, 200, "id5")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 10.01,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

		}
	}

	t.Log("Sim broker: TEST SHORT ORDERS")
	{

		t.Log("Sim broker: test limit order execution on tick")
		{
			t.Log("Normal execution")
			{
				order := newTestOrder(20.02, OrderSell, 200, "ids1")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.07,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					t.Logf("Event: %v", *v)
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}

			}

			t.Log("Partial order fill")
			{
				order := newTestOrder(20.13, OrderSell, 200, "ids2")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				t.Log("First tick without execution")
				{
					tick := marketdata.Tick{
						LastPrice: 20.11,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
						HasTrade:  true,
						HasQuote:  false,
					}

					b.checkOnTickLimit(order, &tick)
					assert.Equal(t, ConfirmedOrder, order.State)
					assert.Len(t, b.confirmedOrders, 1)
					assertNoErrorsGeneratedByBroker(t, b)
					assertNoEventsGeneratedByBroker(t, b)
				}

				t.Log("First fill")
				{
					tick := marketdata.Tick{
						LastPrice: 20.15,
						LastSize:  100,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
						HasTrade:  true,
						HasQuote:  false,
					}

					b.checkOnTickLimit(order, &tick)
					assert.Equal(t, ConfirmedOrder, order.State)
					assert.Len(t, b.confirmedOrders, 1)
					assertNoErrorsGeneratedByBroker(t, b)

					v := getTestSimBrokerGeneratedEvent(t, b)
					assert.NotNil(t, v)
					order.ExecQty = 100

					switch (*v).(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
						assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
					}

				}

				t.Log("Complete fill")
				{
					tick := marketdata.Tick{
						LastPrice: 20.18,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
						HasTrade:  true,
						HasQuote:  false,
					}

					b.checkOnTickLimit(order, &tick)
					assert.Equal(t, ConfirmedOrder, order.State)
					assert.Len(t, b.confirmedOrders, 0)
					assertNoErrorsGeneratedByBroker(t, b)

					v := getTestSimBrokerGeneratedEvent(t, b)
					assert.NotNil(t, v)
					order.ExecQty = 200

					switch (*v).(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
						assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
					}

				}

			}

		}

		t.Log("Sim broker: test strict Limit ordres")
		{
			b.strictLimitOrders = true

			order := newTestOrder(10.02, OrderSell, 200, "ids3")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			t.Log("Tick with order price but without fill")
			{

				tick := marketdata.Tick{
					LastPrice: 10.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoErrorsGeneratedByBroker(t, b)
				assertNoEventsGeneratedByBroker(t, b)
			}

			t.Log("Tick above short order price")
			{
				tick := marketdata.Tick{
					LastPrice: 10.04,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

		}

		t.Log("Sim broker: test not strict Limit orders")
		{
			b.strictLimitOrders = false

			t.Log("Tick with order price with fill")
			{
				order := newTestOrder(10.15, OrderSell, 200, "ids4")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  500,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}

			}

			t.Log("Tick above short order price")
			{
				order := newTestOrder(10.08, OrderSell, 200, "ids5")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 10.09,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 200, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

			t.Log("Tick with order price and partail fill")
			{
				order := newTestOrder(10.15, OrderSell, 200, "ids5")

				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  100,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				order.ExecQty = 100
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}

				tick = marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  100,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickLimit(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v = getTestSimBrokerGeneratedEvent(t, b)
				assert.NotNil(t, v)

				switch (*v).(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, (*v).(*OrderFillEvent).Price)
					assert.Equal(t, 100, (*v).(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, (*v).(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", *v)
				}
			}

		}
	}

}

func TestSimulatedBroker_execs(t *testing.T) {

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
