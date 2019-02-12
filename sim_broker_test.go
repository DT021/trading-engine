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
	b.checkExecutionsOnTicks = true
	errChan := make(chan error)
	eventChan := make(chan event)
	b.Connect(errChan, eventChan)

	return &b
}

func TestSimulatedBroker_Connect(t *testing.T) {
	t.Log("Test connect simulated broker")
	{

		b := SimulatedBroker{}
		errChan := make(chan error)
		eventChan := make(chan event)
		b.Connect(errChan, eventChan)
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
		b.OnNewOrder(&NewOrderEvent{
			LinkedOrder: order,
			BaseEvent:   be(order.Time, order.Symbol)})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 1)

		v := <-eventChan
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

	}

	t.Log("Sim broker: test new order with duplicate ID")
	{
		order := newTestOrder(10.1, OrderSell, 100, "id1")
		b.OnNewOrder(&NewOrderEvent{
			LinkedOrder: order,
			BaseEvent:   be(order.Time, order.Symbol),
		})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 1)
		assert.Len(t, b.allOrders, 1)

		v := <-eventChan
		switch v.(type) {
		case *OrderRejectedEvent:
			assert.Equal(t, "Sim Broker: can't confirm order. Order with this ID already exists on broker side", v.(*OrderRejectedEvent).Reason)
			t.Log("OK! Got OrderRejectedEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderRejectedEvent. Got %v", v.getName())
		}

	}

	t.Log("Sim broker: test new order with wrong params")
	{
		order := newTestOrder(math.NaN(), OrderSell, 100, "id1")
		b.OnNewOrder(&NewOrderEvent{
			LinkedOrder: order,
			BaseEvent:   be(order.Time, order.Symbol),
		})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 1)
		assert.Len(t, b.allOrders, 1)

		v := <-eventChan
		switch v.(type) {
		case *OrderRejectedEvent:
			assert.Equal(t, "Sim Broker: can't confirm order. Order is not valid", v.(*OrderRejectedEvent).Reason)
			t.Log("OK! Got OrderRejectedEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderRejectedEvent. Got %v", v.getName())
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
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		order.State = ConfirmedOrder // Mock it

		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: order.Id})

		assert.Len(t, b.confirmedOrders, 0)
		assert.Len(t, b.canceledOrders, 1)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 1)

		v = <-b.eventChan
		switch v.(type) {
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderCancelEvent. Got %v", v.getName())
		}

	}

	t.Log("Sim Broker: cancel already canceled order")
	{
		ordId := ""
		for k := range b.canceledOrders {
			ordId = k
		}

		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: ordId})
		v := <-b.errChan
		assert.IsType(t, &ErrOrderNotFoundInConfirmedMap{}, v)

	}

	t.Log("Sim broker: cancel not existing order")
	{
		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: "Not existing ID"})
		v := <-b.errChan
		assert.IsType(t, &ErrOrderNotFoundInConfirmedMap{}, v)
	}

	t.Log("Sim broker: cancel order with not confirmed status")
	{
		order := newTestOrder(15, OrderSell, 100, "id2")
		b.OnNewOrder(&NewOrderEvent{
			LinkedOrder: order,
			BaseEvent:   be(order.Time, order.Symbol),
		})

		v := <-b.eventChan
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		b.OnCancelRequest(&OrderCancelRequestEvent{OrdId: "id2"})
		err := <-b.errChan
		assert.IsType(t, &ErrUnexpectedOrderState{}, err)

	}
}

func TestSimulatedBroker_OnReplaceRequest(t *testing.T) {
	b := newTestSimulatedBroker()

	t.Log("Sim Broker: normal replace request")
	{
		order := newTestOrder(15, OrderSell, 100, "1")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 1)

		v := <-b.eventChan
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		order.State = ConfirmedOrder // Mock it

		b.OnReplaceRequest(&OrderReplaceRequestEvent{OrdId: order.Id, NewPrice: 15.5})

		assert.Len(t, b.confirmedOrders, 1)
		assert.Len(t, b.canceledOrders, 0)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 1)

		v = <-b.eventChan
		switch e := v.(type) {
		case *OrderReplacedEvent:
			t.Log("OK! Got OrderReplacedEvent as expected")
			assert.Equal(t, 15.5, e.NewPrice)
			assert.Equal(t, order.Id, e.OrdId)
		default:
			t.Fatalf("Fatal.Expected OrderCancelEvent. Got %v", v.getName())
		}

	}

	t.Log("Sim Broker: replace request with invalid price")
	{
		order := newTestOrder(15, OrderSell, 100, "1_")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order})

		assert.Len(t, b.confirmedOrders, 2)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 2)

		v := <-b.eventChan
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		order.State = ConfirmedOrder // Mock it

		b.OnReplaceRequest(&OrderReplaceRequestEvent{OrdId: order.Id, NewPrice: 0})

		assert.Len(t, b.confirmedOrders, 2)
		assert.Len(t, b.canceledOrders, 0)
		assert.Len(t, b.rejectedOrders, 0)
		assert.Len(t, b.allOrders, 2)

		err := <-b.errChan
		assert.NotNil(t, err)
		assert.IsType(t, &ErrInvalidRequestPrice{}, err)

	}

	t.Log("Sim broker: replace not existing order")
	{
		b.OnReplaceRequest(&OrderReplaceRequestEvent{OrdId: "Not existing ID", NewPrice: 15.0})
		v := <-b.errChan
		assert.IsType(t, &ErrOrderNotFoundInConfirmedMap{}, v)
	}

	t.Log("Sim broker: replace order with not confirmed status")
	{
		order := newTestOrder(15, OrderSell, 100, "id2")
		b.OnNewOrder(&NewOrderEvent{LinkedOrder: order, BaseEvent: be(order.Time, order.Symbol),})

		v := <-b.eventChan
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		b.OnReplaceRequest(&OrderReplaceRequestEvent{OrdId: "id2"})
		err := <-b.errChan
		assert.IsType(t, &ErrUnexpectedOrderState{}, err)

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

func getTestSimBrokerGeneratedEvent(b *SimulatedBroker) event {

	select {
	case v := <-b.eventChan:
		return v

	}
	return nil
}

func getTestSimBrokerGeneratedErrors(b *SimulatedBroker) error {

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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.01, v.(*OrderFillEvent).Price)
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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
				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 19.95, v.(*OrderFillEvent).Price)
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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
				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 21.05, v.(*OrderFillEvent).Price)
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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
				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.01, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v = getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 19.98, v.(*OrderFillEvent).Price)
					assert.Equal(t, 300, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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
				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.09, v.(*OrderFillEvent).Price)
					assert.Equal(t, 600, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v = getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.05, v.(*OrderFillEvent).Price)
					assert.Equal(t, 300, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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
			assert.IsType(t, &ErrInvalidOrder{}, err)
			assertNoEventsGeneratedByBroker(t, b)
		}

		t.Log("Sim broker: put error in chan when order is not in Confirmed or PartialFilledState")
		{
			order := newTestOrder(math.NaN(), OrderSell, 100, "Market7")
			order.Type = MarketOrder
			order.State = NewOrder
			assert.True(t, order.isValid())
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
			assert.IsType(t, &ErrUnexpectedOrderState{}, err)
			assertNoEventsGeneratedByBroker(t, b)
		}

		t.Log("Sim broker: test when we put not market order. should be error")
		{
			order := newTestOrder(20.02, OrderSell, 100, "Market9")

			assert.True(t, order.isValid())
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
			assert.IsType(t, &ErrUnexpectedOrderType{}, err)
			assertNoEventsGeneratedByBroker(t, b)
		}

	}
}

func TestSimulatedBroker_checkOnTickLimit(t *testing.T) {
	b := newTestSimulatedBroker()

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
		err := getTestSimBrokerGeneratedErrors(b)

		assert.NotNil(t, err)
		assert.IsType(t, &ErrUnexpectedOrderType{}, err)

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
		err := getTestSimBrokerGeneratedErrors(b)

		assert.NotNil(t, err)
		assert.IsType(t, &ErrUnexpectedOrderState{}, err)

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
		err := getTestSimBrokerGeneratedErrors(b)
		assert.IsType(t, &ErrInvalidOrder{}, err)

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
		err := getTestSimBrokerGeneratedErrors(b)
		assert.IsType(t, &ErrUnexpectedOrderState{}, err)

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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

					v := getTestSimBrokerGeneratedEvent(b)
					assert.NotNil(t, v)
					order.ExecQty = 100

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

					v := getTestSimBrokerGeneratedEvent(b)
					assert.NotNil(t, v)
					order.ExecQty = 200

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					t.Logf("Event: %v", v)
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

					v := getTestSimBrokerGeneratedEvent(b)
					assert.NotNil(t, v)
					order.ExecQty = 100

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

					v := getTestSimBrokerGeneratedEvent(b)
					assert.NotNil(t, v)
					order.ExecQty = 200

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

					default:

						t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
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

				v = getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

		}
	}

}

func TestSimulatedBroker_checkOnTickStop(t *testing.T) {
	b := newTestSimulatedBroker()
	t.Log("Sim broker: test stop order execution on tick")
	{
		t.Log("LONG orders")
		{
			t.Log("Tick with trade but without quote")
			{
				order := newTestOrder(20.02, OrderBuy, 200, "id1")
				order.Type = StopOrder
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

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick with trade and with quote")
			{
				order := newTestOrder(20.02, OrderBuy, 200, "id2")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.09,
					LastSize:  200,
					BidPrice:  20.08,
					AskPrice:  20.12,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.AskPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick without trade and with quote")
			{
				order := newTestOrder(20.02, OrderBuy, 200, "id3")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: math.NaN(),
					LastSize:  0,
					BidPrice:  20.08,
					AskPrice:  20.12,
					HasTrade:  false,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoErrorsGeneratedByBroker(t, b)
				assertNoEventsGeneratedByBroker(t, b)

				delete(b.confirmedOrders, order.Id)

			}

			t.Log("Tick with price without fill")
			{
				order := newTestOrder(20.02, OrderBuy, 200, "id4")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.00,
					LastSize:  200,
					BidPrice:  20.99,
					AskPrice:  20.12,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoEventsGeneratedByBroker(t, b)
				assertNoErrorsGeneratedByBroker(t, b)

				delete(b.confirmedOrders, order.Id)
			}

			t.Log("Tick with exact order price")
			{
				order := newTestOrder(19.85, OrderBuy, 200, "id5")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 19.85,
					LastSize:  200,
					BidPrice:  20.08,
					AskPrice:  20.12,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.AskPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick with trade and without quote and partial fills")
			{
				order := newTestOrder(20.02, OrderBuy, 500, "id6")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.05,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

				order.ExecQty = int(tick.LastSize)
				order.State = PartialFilledOrder

				tick = marketdata.Tick{
					LastPrice: 20.07,
					LastSize:  900,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, PartialFilledOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v = getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, 300, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}
		}

		t.Log("SHORT orders")
		{
			t.Log("Tick with trade but without quote")
			{
				order := newTestOrder(20.02, OrderSell, 200, "ids1")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 19.98,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick with trade and with quote")
			{
				order := newTestOrder(20.02, OrderSell, 200, "ids2")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 19.95,
					LastSize:  200,
					BidPrice:  19.90,
					AskPrice:  20.12,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.BidPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick without trade and with quote")
			{
				order := newTestOrder(20.02, OrderSell, 200, "ids3")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: math.NaN(),
					LastSize:  0,
					BidPrice:  20.08,
					AskPrice:  20.12,
					HasTrade:  false,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoErrorsGeneratedByBroker(t, b)
				assertNoEventsGeneratedByBroker(t, b)

				delete(b.confirmedOrders, order.Id)

			}

			t.Log("Tick with price without fill")
			{
				order := newTestOrder(20.02, OrderSell, 200, "ids4")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.05,
					LastSize:  200,
					BidPrice:  20.99,
					AskPrice:  20.12,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoEventsGeneratedByBroker(t, b)
				assertNoErrorsGeneratedByBroker(t, b)

				delete(b.confirmedOrders, order.Id)
			}

			t.Log("Tick with exact order price")
			{
				order := newTestOrder(19.85, OrderSell, 200, "ids5")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 19.85,
					LastSize:  200,
					BidPrice:  20.08,
					AskPrice:  20.12,
					HasTrade:  true,
					HasQuote:  true,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.BidPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick with trade and without quote and partial fills")
			{
				order := newTestOrder(20.02, OrderSell, 500, "ids6")
				order.Type = StopOrder
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 20.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 1)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

				order.ExecQty = int(tick.LastSize)
				order.State = PartialFilledOrder

				tick = marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  900,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
				}

				b.checkOnTickStop(order, &tick)
				assert.Equal(t, PartialFilledOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v = getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, 300, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}
		}

	}
}

func TestSimulatedBroker_checkOnTickMOO(t *testing.T) {
	b := newTestSimulatedBroker()
	t.Log("LONG MOO order tests")
	{

		t.Log("Sim broker: test normal execution of MOO")
		{
			order := newTestOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsOpening: true,
			}

			b.checkOnTickMOO(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

			default:

				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Sim broker: tick that is not MOO")
		{
			order := newTestOrder(math.NaN(), OrderBuy, 200, "id2")
			order.Type = MarketOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsOpening: false,
			}

			b.checkOnTickMOO(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 1)
			assertNoErrorsGeneratedByBroker(t, b)
			assertNoEventsGeneratedByBroker(t, b)
			delete(b.confirmedOrders, order.Id)
		}

	}

	t.Log("SHORT MOO order tests")
	{

		t.Log("Sim broker: test normal execution of MOO")
		{
			order := newTestOrder(math.NaN(), OrderSell, 200, "ids1")
			order.Type = MarketOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsOpening: true,
			}

			b.checkOnTickMOO(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

			default:

				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Sim broker: tick that is not MOO")
		{
			order := newTestOrder(math.NaN(), OrderSell, 200, "ids2")
			order.Type = MarketOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsOpening: false,
			}

			b.checkOnTickMOO(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 1)
			assertNoErrorsGeneratedByBroker(t, b)
			assertNoEventsGeneratedByBroker(t, b)
			delete(b.confirmedOrders, order.Id)
		}
	}
}

func TestSimulatedBroker_checkOnTickMOC(t *testing.T) {
	b := newTestSimulatedBroker()
	t.Log("LONG MOC order tests")
	{

		t.Log("Sim broker: test normal execution of MOC")
		{
			order := newTestOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnClose
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsClosing: true,
			}

			b.checkOnTickMOC(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

			default:

				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Sim broker: tick that is not MOC")
		{
			order := newTestOrder(math.NaN(), OrderBuy, 200, "id2")
			order.Type = MarketOnClose
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsClosing: false,
			}

			b.checkOnTickMOC(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 1)
			assertNoErrorsGeneratedByBroker(t, b)
			assertNoEventsGeneratedByBroker(t, b)
			delete(b.confirmedOrders, order.Id)
		}

	}

	t.Log("SHORT MOC order tests")
	{

		t.Log("Sim broker: test normal execution of MOC")
		{
			order := newTestOrder(math.NaN(), OrderSell, 200, "ids1")
			order.Type = MarketOnClose
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsClosing: true,
			}

			b.checkOnTickMOC(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

			default:

				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Sim broker: tick that is not MOC")
		{
			order := newTestOrder(math.NaN(), OrderSell, 200, "ids2")
			order.Type = MarketOnClose
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  true,
				HasQuote:  true,
				IsClosing: false,
			}

			b.checkOnTickMOC(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 1)
			assertNoErrorsGeneratedByBroker(t, b)
			assertNoEventsGeneratedByBroker(t, b)
			delete(b.confirmedOrders, order.Id)
		}
	}
}

func TestSimulatedBroker_checkOnTickLimitAuction(t *testing.T) {
	b := newTestSimulatedBroker()
	b.marketOpenUntilTime = TimeOfDay{9, 33, 0}
	b.marketCloseUntilTime = TimeOfDay{16, 03, 0}
	t.Log("Sim broker: test auction orders LONG")
	{
		t.Log("Complete execution")
		{
			order := newTestOrder(15.87, OrderBuy, 200, "id1")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.80,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				HasTrade:  true,
				HasQuote:  false,
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

			default:

				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Tick without  execution")
		{
			order := newTestOrder(15.87, OrderBuy, 200, "id2")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.90,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				HasTrade:  true,
				HasQuote:  false,
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderCancelEvent:
				t.Log("OK! Got OrderFillEvent as expected")

				assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)

			default:

				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("strict and not strict limit orders execution")
		{
			t.Log("strict")
			{
				b.strictLimitOrders = true
				order := newTestOrder(15.87, OrderBuy, 200, "st10")
				order.Type = LimitOnOpen
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
					IsOpening: true,
				}

				b.checkOnTickLimitAuction(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderCancelEvent:
					t.Log("OK! Got OrderCancelEvent as expected")
					assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}
			}

			t.Log("not strict")
			{
				b.strictLimitOrders = false
				order := newTestOrder(15.87, OrderBuy, 200, "st2")
				order.Type = LimitOnOpen
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
					IsOpening: true,
				}

				b.checkOnTickLimitAuction(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}
			}

		}

		t.Log("Partial fill")
		{
			order := newTestOrder(15.87, OrderBuy, 1000, "id1z")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.80,
				LastSize:  389,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				HasTrade:  true,
				HasQuote:  false,
				IsOpening: true,
				Datetime:  time.Date(2012, 1, 2, 9, 32, 0, 0, time.UTC),
			}

			b.checkOnTickLimitAuction(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, 389, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
			case *OrderCancelEvent:
				assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)

			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v, %v", v, v.(event).getName())
			}

			v = getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, 389, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
			case *OrderCancelEvent:
				assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)

			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v, %v", v, v.(event).getName())
			}
		}
	}

	t.Log("Sim broker: test auction orders SHORT")
	{
		t.Log("Complete execution")
		{
			order := newTestOrder(15.87, OrderSell, 200, "ids1")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.90,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				HasTrade:  true,
				HasQuote:  false,
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

			default:

				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Tick without  execution")
		{
			order := newTestOrder(15.87, OrderSell, 200, "id2s")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.20,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				HasTrade:  true,
				HasQuote:  false,
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderCancelEvent:
				t.Log("OK! Got OrderCancelEvent as expected")
				assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("strict and not strict limit orders execution")
		{
			t.Log("strict")
			{
				b.strictLimitOrders = true
				order := newTestOrder(15.87, OrderSell, 200, "st1s")
				order.Type = LimitOnOpen
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
					IsOpening: true,
				}

				b.checkOnTickLimitAuction(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderCancelEvent:
					t.Log("OK! Got OrderCancelEvent as expected")
					assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}
			}

			t.Log("not strict")
			{
				b.strictLimitOrders = false
				order := newTestOrder(15.87, OrderSell, 200, "st2s")
				order.Type = LimitOnOpen
				order.State = ConfirmedOrder
				assert.True(t, order.isValid())
				b.confirmedOrders[order.Id] = order

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					HasTrade:  true,
					HasQuote:  false,
					IsOpening: true,
				}

				b.checkOnTickLimitAuction(order, &tick)
				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)
				assertNoErrorsGeneratedByBroker(t, b)

				v := getTestSimBrokerGeneratedEvent(b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, order.Qty, v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

				default:

					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}
			}

		}

		t.Log("Partial fill")
		{
			order := newTestOrder(15.87, OrderSell, 1000, "id1v")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.confirmedOrders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.92,
				LastSize:  378,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				HasTrade:  true,
				HasQuote:  false,
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
			assertNoErrorsGeneratedByBroker(t, b)

			v := getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, 378, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
			case *OrderCancelEvent:
				assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}

			v = getTestSimBrokerGeneratedEvent(b)
			assert.NotNil(t, v)

			switch v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
				assert.Equal(t, 378, v.(*OrderFillEvent).Qty)
				assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
			case *OrderCancelEvent:
				assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}
	}
}

func TestSimulatedBroker_checkOnTickLOC(t *testing.T) {
	b := newTestSimulatedBroker()
	b.marketCloseUntilTime = TimeOfDay{16, 5, 0}
	t.Log("Sim broker: check LOC cancelation by time")
	{
		order := newTestOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnClose
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.confirmedOrders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 16, 7, 0, 0, time.UTC),
		}

		b.checkOnTickLOC(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
			assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)
		default:

			t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
		}
	}

	t.Log("Sim broker: check LOC hold by time. ")
	{
		order := newTestOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnClose
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.confirmedOrders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 16, 3, 0, 0, time.UTC),
		}

		b.checkOnTickLOC(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)
		assertNoErrorsGeneratedByBroker(t, b)
		assertNoEventsGeneratedByBroker(t, b)

	}

}

func TestSimulatedBroker_checkOnTickLOO(t *testing.T) {
	b := newTestSimulatedBroker()
	b.marketOpenUntilTime = TimeOfDay{9, 35, 0}
	t.Log("Sim broker: check LOO cancelation by time")
	{
		order := newTestOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.confirmedOrders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 35, 1, 0, time.UTC),
		}

		b.checkOnTickLOO(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
			assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)
		default:

			t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
		}
	}

	t.Log("Sim broker: check LOO hold by time. ")
	{
		order := newTestOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.confirmedOrders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 31, 0, 0, time.UTC),
		}

		b.checkOnTickLOO(order, &tick)
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)
		assertNoErrorsGeneratedByBroker(t, b)
		assertNoEventsGeneratedByBroker(t, b)

	}

}

func TestSimulatedBroker_OnTick(t *testing.T) {
	b := newTestSimulatedBroker()
	b.strictLimitOrders = true
	b.marketOpenUntilTime = TimeOfDay{9, 35, 0}
	b.marketCloseUntilTime = TimeOfDay{16, 5, 15}

	putNewOrder := func(price float64, ordType OrderType, ordSide OrderSide, qty int, id string) *Order {
		order := newTestOrder(price, ordSide, qty, id)
		order.State = ConfirmedOrder
		order.Type = ordType
		assert.True(t, order.isValid())
		prevLen := len(b.confirmedOrders)
		b.confirmedOrders[order.Id] = order
		assert.Len(t, b.confirmedOrders, prevLen+1)
		return order
	}

	order1 := putNewOrder(math.NaN(), MarketOrder, OrderBuy, 200, "1")
	order2 := putNewOrder(20.05, LimitOrder, OrderSell, 120, "2")
	order3 := putNewOrder(math.NaN(), MarketOnOpen, OrderSell, 300, "3")
	order4 := putNewOrder(10.03, LimitOnClose, OrderBuy, 200, "4")
	order5 := putNewOrder(50.08, StopOrder, OrderBuy, 90, "5")
	order6 := putNewOrder(19.08, LimitOnOpen, OrderBuy, 90, "6")
	order7 := putNewOrder(20.08, LimitOnClose, OrderSell, 90, "7")

	t.Log("Sim Broker: OnTick. Reaction on broken tick. Error expected")
	{
		tick := marketdata.Tick{
			Symbol:    "Test",
			LastPrice: math.NaN(),
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}

		assert.False(t, b.tickIsValid(&tick))

		prevLen := len(b.confirmedOrders)
		b.OnTick(&tick)
		assert.Len(t, b.confirmedOrders, prevLen)
		assertNoEventsGeneratedByBroker(t, b)

		err := getTestSimBrokerGeneratedErrors(b)
		assert.NotNil(t, err)
		assert.IsType(t, &ErrBrokenTick{}, err)
	}

	t.Log("Sim Broker: OnTick. First tick - execute only market")
	{
		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}

		prevLen := len(b.confirmedOrders)
		prevLenFills := len(b.filledOrders)
		b.OnTick(&tick)
		assert.Len(t, b.confirmedOrders, prevLen-1)
		assert.Len(t, b.filledOrders, prevLenFills+1)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
			assert.Equal(t, order1.Qty, v.(*OrderFillEvent).Qty)
			assert.Equal(t, order1.Id, v.(*OrderFillEvent).OrdId)

		default:

			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

	}

	t.Log("Sim Broker: OnTick. Second tick - execute nothing")
	{
		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}
		prevLen := len(b.confirmedOrders)
		b.OnTick(&tick)
		assert.Len(t, b.confirmedOrders, prevLen)
		assertNoErrorsGeneratedByBroker(t, b)
		assertNoEventsGeneratedByBroker(t, b)

	}

	t.Log("Sim Broker: OnTick. Third tick - execute only sell limit")
	{
		tick := marketdata.Tick{
			LastPrice: 20.06,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 31, 1, 0, time.UTC),
		}
		prevLen := len(b.confirmedOrders)
		prevLenFills := len(b.filledOrders)
		b.OnTick(&tick)
		assert.Len(t, b.confirmedOrders, prevLen-1)
		assert.Len(t, b.filledOrders, prevLenFills+1)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, order2.Price, v.(*OrderFillEvent).Price)
			assert.Equal(t, order2.Qty, v.(*OrderFillEvent).Qty)
			assert.Equal(t, order2.Id, v.(*OrderFillEvent).OrdId)

		default:

			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

	}

	t.Log("Sim Broker: OnTick. Forth tick - execute on open orders")
	{
		tick := marketdata.Tick{
			LastPrice: 15.88,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: true,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}
		prevLen := len(b.confirmedOrders)
		prevLenFills := len(b.filledOrders)
		b.OnTick(&tick)
		assert.Len(t, b.confirmedOrders, prevLen-2)
		assert.Len(t, b.filledOrders, prevLenFills+2)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)
		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			e := v.(*OrderFillEvent)
			assert.Equal(t, tick.LastPrice, e.Price)
			if e.OrdId != order3.Id {
				assert.Equal(t, order6.Qty, e.Qty)
				assert.Equal(t, order6.Id, e.OrdId)

			} else {
				assert.Equal(t, order3.Qty, e.Qty)
				assert.Equal(t, order3.Id, e.OrdId)
			}

		default:

			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

		v = getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)
		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			e := v.(*OrderFillEvent)
			assert.Equal(t, tick.LastPrice, e.Price)
			if e.OrdId != order3.Id {
				assert.Equal(t, order6.Qty, e.Qty)
				assert.Equal(t, order6.Id, e.OrdId)

			} else {
				assert.Equal(t, order3.Qty, e.Qty)
				assert.Equal(t, order3.Id, e.OrdId)
			}

		default:

			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

	}

	t.Log("Sim Broker: OnTick. Fifth tick - execute stop")
	{
		tick := marketdata.Tick{
			LastPrice: 50.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 15, 30, 1, 0, time.UTC),
		}
		prevLen := len(b.confirmedOrders)
		prevLenFills := len(b.filledOrders)
		b.OnTick(&tick)
		assert.Len(t, b.confirmedOrders, prevLen-1)
		assert.Len(t, b.filledOrders, prevLenFills+1)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
			assert.Equal(t, order5.Qty, v.(*OrderFillEvent).Qty)
			assert.Equal(t, order5.Id, v.(*OrderFillEvent).OrdId)

		default:

			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

	}

	t.Log("Sim Broker: OnTick. Sixth tick - execute one on close and cancel another")
	{
		tick := marketdata.Tick{
			LastPrice: 10,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsClosing: true,
			Datetime:  time.Date(2010, 5, 5, 16, 01, 1, 0, time.UTC),
		}
		prevLen := len(b.confirmedOrders)
		prevLenFills := len(b.filledOrders)
		b.OnTick(&tick)
		assert.Len(t, b.confirmedOrders, prevLen-2)
		assert.Len(t, b.filledOrders, prevLenFills+1)
		assert.Len(t, b.canceledOrders, 1)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
			assert.Equal(t, order4.Qty, v.(*OrderFillEvent).Qty)
			assert.Equal(t, order4.Id, v.(*OrderFillEvent).OrdId)
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
			assert.Equal(t, order7.Id, v.(*OrderCancelEvent).OrdId)

		default:

			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

		v = getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
			assert.Equal(t, order4.Qty, v.(*OrderFillEvent).Qty)
			assert.Equal(t, order4.Id, v.(*OrderFillEvent).OrdId)
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
			assert.Equal(t, order7.Id, v.(*OrderCancelEvent).OrdId)

		default:

			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

	}

	t.Log("Sim Broker: OnTick. React when we don't have confirmed orders")
	{
		assert.Len(t, b.confirmedOrders, 0)
		tick := marketdata.Tick{
			LastPrice: 10,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			HasTrade:  true,
			HasQuote:  false,
			IsClosing: false,
			Datetime:  time.Date(2010, 5, 5, 16, 01, 1, 0, time.UTC),
		}

		b.OnTick(&tick)

		assertNoErrorsGeneratedByBroker(t, b)
		assertNoEventsGeneratedByBroker(t, b)

	}

}
