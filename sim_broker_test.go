package engine

import (
	"alex/marketdata"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math"
	"sync"
	"testing"
	"time"
)

func newTestSimBroker() *SimBroker {
	b := SimBroker{delay: 1000}
	b.checkExecutionsOnTicks = true
	errChan := make(chan error)
	b.Init(errChan, []string{""})
	return &b
}

func newTestSimBrokerWorker() *simBrokerWorker {
	bsc := BrokerSymbolChannels{
		signals:        make(chan event),
		broker:         make(chan event),
		brokerNotifier: make(chan struct{}),
		notifyBroker:   make(chan *BrokerNotifyEvent),
	}
	w := simBrokerWorker{
		symbol:               "Test",
		errChan:              make(chan error),
		ch:                   bsc,
		terminationChan:      make(chan struct{}),
		delay:                100,
		strictLimitOrders:    false,
		marketOpenUntilTime:  TimeOfDay{9, 35, 0},
		marketCloseUntilTime: TimeOfDay{16, 5, 0},
		mpMutext:             &sync.RWMutex{},
		orders:               make(map[string]*simBrokerOrder),
	}

	return &w
}

func TestSimulatedBroker_Init(t *testing.T) {
	t.Log("Test connect simulated broker")
	{
		b := SimBroker{}
		errChan := make(chan error)
		symbols := []string{"Test1", "Test2"}

		b.Init(errChan, symbols)
		for _, s := range symbols {
			_, ok := b.workers[s]
			assert.True(t, ok)
		}
	}

}

func getFromSimBrokerWorkerChan(ch chan event) event {
	select {
	case e := <-ch:
		return e
	case <-time.After(1 * time.Second):
		return nil
	}

}

func TestSimulatedBroker_OnNewOrder(t *testing.T) {
	b := newTestSimBrokerWorker()

	t.Log("Sim broker: test normal new order")
	{
		order := newTestOrder(10.1, OrderSell, 100, "id1")
		v := putNewOrderToWorkerAndGetBrokerEvent(b, order)
		assert.NotNil(t, v)
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		assert.Len(t, b.orders, 1)
		assert.Equal(t, ConfirmedOrder, b.orders[order.Id].BrokerState)

	}

	t.Log("Sim broker: test new order with duplicate ID")
	{
		order := newTestOrder(10.1, OrderSell, 100, "id1")
		v := putNewOrderToWorkerAndGetBrokerEvent(b, order)
		assert.NotNil(t, v)
		switch v.(type) {
		case *OrderRejectedEvent:
			t.Log("OK! Got OrderRejectedEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		assert.Len(t, b.orders, 1)
		assert.Equal(t, ConfirmedOrder, b.orders[order.Id].BrokerState)

	}

	t.Log("Sim broker: test new order with wrong params")
	{
		order := newTestOrder(math.NaN(), OrderSell, 100, "id1==")
		v := putNewOrderToWorkerAndGetBrokerEvent(b, order)
		assert.NotNil(t, v)
		switch v.(type) {
		case *OrderRejectedEvent:
			t.Log("OK! Got OrderRejectedEvent as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		assert.Len(t, b.orders, 2)
		assert.Equal(t, RejectedOrder, b.orders[order.Id].BrokerState)
	}

}

func putNewOrderToWorkerAndGetBrokerEvent(w *simBrokerWorker, order *Order) event {
	wg := &sync.WaitGroup{}
	go func() {
		wg.Add(1)
		w.onNewOrder(&NewOrderEvent{
			LinkedOrder: order,
			BaseEvent:   be(order.Time, order.Symbol)})
		wg.Done()
	}()

	v := getFromSimBrokerWorkerChan(w.ch.broker)
	wg.Wait()

	return v
}

func putCancelRequestToWorkerAndGetBrokerEvent(w *simBrokerWorker, orderId string) event {
	wg := &sync.WaitGroup{}
	go func() {
		wg.Add(1)
		w.onCancelRequest(&OrderCancelRequestEvent{OrdId: orderId})
		wg.Done()
	}()

	v := getFromSimBrokerWorkerChan(w.ch.broker)
	wg.Wait()

	return v
}

func putReplaceRequestToWorkerAndGetBrokerEvent(w *simBrokerWorker, orderId string, newPrice float64) event {
	wg := &sync.WaitGroup{}
	go func() {
		wg.Add(1)
		w.onReplaceRequest(&OrderReplaceRequestEvent{OrdId: orderId, NewPrice: newPrice})
		wg.Done()
	}()

	v := getFromSimBrokerWorkerChan(w.ch.broker)
	wg.Wait()

	return v
}

func TestSimulatedBroker_OnCancelRequest(t *testing.T) {
	b := newTestSimBrokerWorker()

	t.Log("Sim Broker: normal cancel request")
	{
		order := newTestOrder(15, OrderSell, 100, "1")
		v := putNewOrderToWorkerAndGetBrokerEvent(b, order)
		assert.NotNil(t, v)
		switch v.(type) {
		case *OrderConfirmationEvent:
			t.Log("OK! Got confirmation event as expected")
		default:
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %v", v.getName())
		}

		assert.Len(t, b.orders, 1)
		assert.Equal(t, ConfirmedOrder, b.orders[order.Id].BrokerState)

		v = putCancelRequestToWorkerAndGetBrokerEvent(b, order.Id)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
		default:
			if v != nil {
				t.Fatalf("Fatal.Expected OrderCancelEvent. Got %v", v.getName())
			} else {
				t.Fatal("Fatal.Expected OrderCancelEvent. Got nil")
			}
		}

		assert.Len(t, b.orders, 1)
		assert.Equal(t, CanceledOrder, b.orders[order.Id].BrokerState)

	}

	t.Log("Sim Broker: cancel already canceled order")
	{
		ordId := ""
		for id_, k := range b.orders {
			if k.BrokerState != CanceledOrder {
				continue
			}
			ordId = id_
		}

		v := putCancelRequestToWorkerAndGetBrokerEvent(b, ordId)
		assert.IsType(t, &OrderCancelRejectEvent{}, v)
	}

	t.Log("Sim broker: cancel not existing order")
	{
		v := putCancelRequestToWorkerAndGetBrokerEvent(b, "Not existing ID")
		assert.IsType(t, &OrderCancelRejectEvent{}, v)

	}

}

func TestSimulatedBroker_OnReplaceRequest(t *testing.T) {
	b := newTestSimBrokerWorker()

	t.Log("Sim Broker: normal replace request")
	{
		order := newTestOrder(15, OrderSell, 100, "1")
		v := putNewOrderToWorkerAndGetBrokerEvent(b, order)

		assert.IsType(t, &OrderConfirmationEvent{}, v)
		assert.Len(t, b.orders, 1)
		assert.Equal(t, ConfirmedOrder, b.orders[order.Id].BrokerState)

		v = putReplaceRequestToWorkerAndGetBrokerEvent(b, order.Id, 15.5)
		assert.NotNil(t, v)
		assert.Len(t, b.orders, 1)
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
		v := putNewOrderToWorkerAndGetBrokerEvent(b, order)
		assert.IsType(t, &OrderConfirmationEvent{}, v)
		assert.Len(t, b.orders, 2)

		v = putReplaceRequestToWorkerAndGetBrokerEvent(b, order.Id, 0)
		assert.NotNil(t, v)
		assert.IsType(t, &OrderReplaceRejectEvent{}, v)
		assert.Len(t, b.orders, 2)

	}

	t.Log("Sim broker: replace not existing order")
	{
		v := putReplaceRequestToWorkerAndGetBrokerEvent(b, "Not Existing", 20)
		assert.NotNil(t, v)
		assert.IsType(t, &OrderReplaceRejectEvent{}, v)
	}

	t.Log("Sim broker: replace order with not confirmed status")
	{
		order := newTestOrder(15, OrderSell, 100, "id2")
		v := putNewOrderToWorkerAndGetBrokerEvent(b, order)
		assert.IsType(t, &OrderConfirmationEvent{}, v)

		v = putCancelRequestToWorkerAndGetBrokerEvent(b, order.Id)
		assert.IsType(t, &OrderCancelEvent{}, v)
		assert.Equal(t, CanceledOrder, b.orders[order.Id].BrokerState)

		v = putReplaceRequestToWorkerAndGetBrokerEvent(b, order.Id, 99)
		assert.NotNil(t, v)
		assert.IsType(t, &OrderReplaceRejectEvent{}, v)
		assert.Equal(t, 15.0, b.orders[order.Id].Price)
	}
}

func getTestSimBrokerGeneratedErrors(b *simBrokerWorker) error {

	select {
	case v := <-b.errChan:
		return v
	case <-time.After(1 * time.Second):
		return nil

	}

}

func newTestBrokerOrder(price float64, side OrderSide, qty int64, id string) *simBrokerOrder {
	ord := newTestOrder(price, side, qty, id)
	o := simBrokerOrder{Order: ord, BrokerState: ConfirmedOrder}
	return &o
}

func assertNoErrorsGeneratedByBroker(t *testing.T, b *simBrokerWorker) {
	select {
	case v := <-b.errChan:
		t.Errorf("ERROR! Expected no errors. Found: %v", v)
	default:
		t.Log("OK! Error chan is empty")
		return
	}
}

func TestSimulatedBroker_checkOnTickMarket(t *testing.T) {
	b := newTestSimBrokerWorker()

	t.Log("Case when sim broker has only trades feed")
	{
		t.Log("Sim broker: test normal market order execution on tick")
		{
			t.Log("Check execution when we have only last trade price")
			{

				order := newTestBrokerOrder(math.NaN(), OrderSell, 100, "Market1")
				order.Type = MarketOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickMarket(order, &tick)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.01, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

			}

		}
	}

	t.Log("Case when sim broker has both quotes and trades feed. ")
	{

		t.Log("Sim broker: normal execution on tick with quotes")
		{
			t.Log("SHORT ORDER full execution")
			{

				order := newTestBrokerOrder(math.NaN(), OrderSell, 100, "Market3")
				order.Type = MarketOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.95,
					AskPrice:  20.05,
					BidSize:   200,
					AskSize:   300,
				}

				v := b.checkOnTickMarket(order, &tick)

				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 19.95, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("LONG ORDER full execution")
			{

				order := newTestBrokerOrder(math.NaN(), OrderBuy, 100, "Market4")
				order.Type = MarketOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.95,
					AskPrice:  21.05,
					BidSize:   200,
					AskSize:   300,
				}

				v := b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 21.05, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

			}

			t.Log("SHORT ORDER partial execution")
			{

				order := newTestBrokerOrder(math.NaN(), OrderSell, 500, "Market5")
				order.Type = MarketOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.01,
					AskPrice:  20.05,
					BidSize:   200,
					AskSize:   300,
				}

				v := b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.01, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

				//New tick - fill rest of the order. New price*************************
				order.BrokerExecQty = 200
				order.BrokerState = PartialFilledOrder

				tick = marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.98,
					AskPrice:  20.05,
					BidSize:   800,
					AskSize:   300,
				}

				v = b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 19.98, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(300), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("LONG ORDER partial execution")
			{

				order := newTestBrokerOrder(math.NaN(), OrderBuy, 900, "Market6")
				order.Type = MarketOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.01,
					AskPrice:  20.09,
					BidSize:   200,
					AskSize:   600,
				}

				v := b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.09, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(600), v.(*OrderFillEvent).Qty)
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
				}

				v = b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, 20.05, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(300), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

		}
		t.Log("Sim broker: execute only when we got tick with prices")
		{
			order := newTestBrokerOrder(math.NaN(), OrderSell, 100, "Market7")
			order.Type = MarketOrder
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				BidSize:   0,
				AskSize:   0,
			}

			v := b.checkOnTickMarket(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
			switch i := v.(type) {
			case *OrderFillEvent:
				assert.Equal(t, i.Price, tick.LastPrice)
			default:
				t.Errorf("Unexpected type: %v", i)

			}

		}

		t.Log("Sim broker: put error in chan when order is not valid")
		{
			order := newTestBrokerOrder(20.0, OrderSell, 100, "Market7")
			order.Type = MarketOrder
			assert.False(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  19.95,
				AskPrice:  20.01,
				BidSize:   100,
				AskSize:   200,
			}

			v := b.checkOnTickMarket(order, &tick)
			assert.Nil(t, v)
			select {
			case err := <-b.errChan:
				assert.NotNil(t, err)
				assert.IsType(t, &ErrInvalidOrder{}, err)
			case <-time.After(time.Millisecond):
				t.Error("Expected error")
			}

		}

		t.Log("Sim broker: test when we put not market order. should be error")
		{
			order := newTestBrokerOrder(20.02, OrderSell, 100, "Market9")

			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  19.95,
				AskPrice:  20.01,
				BidSize:   100,
				AskSize:   200,
			}

			v := b.checkOnTickMarket(order, &tick)
			assert.Nil(t, v)

			select {
			case err := <-b.errChan:
				assert.NotNil(t, err)
				assert.IsType(t, &ErrUnexpectedOrderType{}, err)
			case <-time.After(time.Millisecond):
				t.Error("Expected error")
			}

		}

	}
}

func TestSimulatedBroker_checkOnTickLimit(t *testing.T) {
	b := newTestSimBrokerWorker()

	t.Log("Sim broker: test limit exectution for not limit order")
	{
		order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "Wid1")
		order.Type = MarketOrder

		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		e := b.checkOnTickLimit(order, &tick)
		assert.Nil(t, e)

		err := getTestSimBrokerGeneratedErrors(b)

		assert.NotNil(t, err)
		assert.IsType(t, &ErrUnexpectedOrderType{}, err)

	}

	t.Log("Sim broker: test limit execution for already filled order")
	{
		order := newTestBrokerOrder(20.01, OrderBuy, 200, "Wid10")

		order.BrokerState = FilledOrder
		order.BrokerExecQty = 200
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		e := b.checkOnTickLimit(order, &tick)
		assert.Nil(t, e)
		err := getTestSimBrokerGeneratedErrors(b)

		assert.NotNil(t, err)
		assert.IsType(t, &ErrUnexpectedOrderState{}, err)

	}

	t.Log("Sim broker: test limit execution for not valid order")
	{
		order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "Wid2")
		assert.False(t, order.isValid())
		order.BrokerState = ConfirmedOrder

		tick := marketdata.Tick{
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		e := b.checkOnTickLimit(order, &tick)
		assert.Nil(t, e)
		err := getTestSimBrokerGeneratedErrors(b)
		assert.IsType(t, &ErrInvalidOrder{}, err)
	}

	t.Log("Sim broker: test limit execution for tick without trade")
	{
		order := newTestBrokerOrder(20, OrderBuy, 200, "Wid3")
		assert.True(t, order.isValid())

		//Case when tick has tag but don't have price
		tick := marketdata.Tick{
			LastPrice: math.NaN(),
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		e := b.checkOnTickLimit(order, &tick)
		assert.Nil(t, e)
		assertNoErrorsGeneratedByBroker(t, b)

		//Case when tick has right Tag
		tick = marketdata.Tick{
			LastPrice: math.NaN(),
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		e = b.checkOnTickLimit(order, &tick)
		assert.Nil(t, e)
		assertNoErrorsGeneratedByBroker(t, b)
	}

	t.Log("Sim broker: test limit execution for not confirmed order")
	{
		order := newTestBrokerOrder(20, OrderBuy, 200, "Wid4")
		assert.True(t, order.isValid())
		order.BrokerState = NewOrder
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 19.88,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		e := b.checkOnTickLimit(order, &tick)
		assert.Equal(t, NewOrder, order.BrokerState)
		assert.Nil(t, e)
		err := getTestSimBrokerGeneratedErrors(b)
		assert.IsType(t, &ErrUnexpectedOrderState{}, err)
	}

	t.Log("Sim broker: TEST LONG ORDERS")
	{

		t.Log("Sim broker: test limit order execution on tick")
		{
			t.Log("Normal execution")
			{
				order := newTestBrokerOrder(20.02, OrderBuy, 200, "id1")
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.00,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)

				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

			}

			t.Log("Partial order fill")
			{
				order := newTestBrokerOrder(20.13, OrderBuy, 200, "id2")
				assert.True(t, order.isValid())

				t.Log("First tick without execution")
				{
					tick := marketdata.Tick{
						LastPrice: 20.15,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					e := b.checkOnTickLimit(order, &tick)
					assertNoErrorsGeneratedByBroker(t, b)
					assert.Nil(t, e)
				}

				t.Log("First fill")
				{
					tick := marketdata.Tick{
						LastPrice: 20.12,
						LastSize:  100,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					v := b.checkOnTickLimit(order, &tick)
					assertNoErrorsGeneratedByBroker(t, b)
					assert.NotNil(t, v)
					order.BrokerExecQty = 100

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
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
					}

					v := b.checkOnTickLimit(order, &tick)
					assertNoErrorsGeneratedByBroker(t, b)
					assert.NotNil(t, v)
					order.BrokerExecQty = 200

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
					default:
						t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
					}
				}
			}
		}

		t.Log("Sim broker: test strict limit orders execution")
		{
			b.strictLimitOrders = true

			order := newTestBrokerOrder(10.02, OrderBuy, 200, "id3")
			assert.True(t, order.isValid())

			t.Log("Tick with order price but without fill")
			{
				tick := marketdata.Tick{
					LastPrice: 10.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.Nil(t, v)
			}

			t.Log("Tick below order price")
			{
				tick := marketdata.Tick{
					LastPrice: 10.01,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
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
				order := newTestBrokerOrder(10.15, OrderBuy, 200, "id4")
				assert.True(t, order.isValid())
				tick := marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  500,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick below order price")
			{
				order := newTestBrokerOrder(10.08, OrderBuy, 200, "id5")
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 10.01,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
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
				order := newTestBrokerOrder(20.02, OrderSell, 200, "ids1")

				tick := marketdata.Tick{
					LastPrice: 20.07,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					t.Logf("Event: %v", v)
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Partial order fill")
			{
				order := newTestBrokerOrder(20.13, OrderSell, 200, "ids2")

				t.Log("First tick without execution")
				{
					tick := marketdata.Tick{
						LastPrice: 20.11,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					v := b.checkOnTickLimit(order, &tick)
					assert.Nil(t, v)
					assertNoErrorsGeneratedByBroker(t, b)
				}

				t.Log("First fill")
				{
					tick := marketdata.Tick{
						LastPrice: 20.15,
						LastSize:  100,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					v := b.checkOnTickLimit(order, &tick)
					assertNoErrorsGeneratedByBroker(t, b)
					assert.NotNil(t, v)
					order.BrokerExecQty = 100

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
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
					}

					v := b.checkOnTickLimit(order, &tick)
					assertNoErrorsGeneratedByBroker(t, b)
					assert.NotNil(t, v)
					order.BrokerExecQty = 200

					switch v.(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
						assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
						assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
					default:
						t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
					}
				}
			}
		}

		t.Log("Sim broker: test strict limit orders execution")
		{
			b.strictLimitOrders = true

			order := newTestBrokerOrder(10.02, OrderSell, 200, "ids3")

			t.Log("Tick with order price but without fill")
			{

				tick := marketdata.Tick{
					LastPrice: 10.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assert.Nil(t, v)
				assertNoErrorsGeneratedByBroker(t, b)
			}

			t.Log("Tick above short order price")
			{
				tick := marketdata.Tick{
					LastPrice: 10.04,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
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
				order := newTestBrokerOrder(10.15, OrderSell, 200, "ids4")
				tick := marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  500,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick above short order price")
			{
				order := newTestBrokerOrder(10.08, OrderSell, 200, "ids5")
				tick := marketdata.Tick{
					LastPrice: 10.09,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick with order price and partail fill")
			{
				order := newTestBrokerOrder(10.15, OrderSell, 200, "ids6")
				tick := marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  100,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickLimit(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)
				order.BrokerExecQty = 100

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

				tick = marketdata.Tick{
					LastPrice: 10.15,
					LastSize:  100,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v = b.checkOnTickLimit(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(100), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}
		}
	}
}

func TestSimulatedBroker_checkOnTickStop(t *testing.T) {
	b := newTestSimBrokerWorker()
	t.Log("Sim broker: test stop order execution on tick")
	{
		t.Log("Sim broker: test stop execution for not stop order")
		{
			order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "Wid1")
			order.Type = MarketOrder
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			e := b.checkOnTickStop(order, &tick)
			assert.Nil(t, e)
			err := getTestSimBrokerGeneratedErrors(b)
			assert.NotNil(t, err)
			assert.IsType(t, &ErrUnexpectedOrderType{}, err)

		}

		t.Log("Sim broker: test stop order execution for already filled order")
		{
			order := newTestBrokerOrder(20.01, OrderBuy, 200, "Wid10")
			order.Type = StopOrder
			order.BrokerState = FilledOrder
			order.BrokerExecQty = 200
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			e := b.checkOnTickStop(order, &tick)
			assert.Nil(t, e)
			err := getTestSimBrokerGeneratedErrors(b)

			assert.NotNil(t, err)
			assert.IsType(t, &ErrUnexpectedOrderState{}, err)

		}

		t.Log("Sim broker: test stop execution for not valid order")
		{
			order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "Wid2")
			order.Type = StopOrder
			assert.False(t, order.isValid())
			order.BrokerState = ConfirmedOrder

			tick := marketdata.Tick{
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			e := b.checkOnTickStop(order, &tick)
			assert.Nil(t, e)
			err := getTestSimBrokerGeneratedErrors(b)
			assert.IsType(t, &ErrInvalidOrder{}, err)
		}

		t.Log("Sim broker: test stop execution for tick without trade")
		{
			order := newTestBrokerOrder(20, OrderBuy, 200, "Wid3")
			order.Type = StopOrder
			assert.True(t, order.isValid())

			//Case when tick has tag but don't have price
			tick := marketdata.Tick{
				LastPrice: math.NaN(),
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			e := b.checkOnTickStop(order, &tick)
			assert.Nil(t, e)
			assertNoErrorsGeneratedByBroker(t, b)

			//Case when tick has right Tag
			tick = marketdata.Tick{
				LastPrice: math.NaN(),
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			e = b.checkOnTickStop(order, &tick)
			assert.Nil(t, e)
			assertNoErrorsGeneratedByBroker(t, b)
		}

		t.Log("Sim broker: test stop execution for not confirmed order")
		{
			order := newTestBrokerOrder(20, OrderBuy, 200, "Wid4")
			order.Type = StopOrder
			assert.True(t, order.isValid())
			order.BrokerState = NewOrder
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 19.88,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			e := b.checkOnTickStop(order, &tick)
			assert.Equal(t, NewOrder, order.BrokerState)
			assert.Nil(t, e)
			err := getTestSimBrokerGeneratedErrors(b)
			assert.IsType(t, &ErrUnexpectedOrderState{}, err)
		}

		t.Log("LONG orders")
		{
			t.Log("Tick with trade but without quote")
			{
				order := newTestBrokerOrder(20.02, OrderBuy, 200, "id1")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.07,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick with trade and with quote")
			{
				order := newTestBrokerOrder(20.02, OrderBuy, 200, "id2")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.09,
					LastSize:  200,
					BidPrice:  20.08,
					BidSize:   200,
					AskSize:   200,
					AskPrice:  20.12,
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
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
				order := newTestBrokerOrder(20.02, OrderBuy, 200, "id3")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: math.NaN(),
					LastSize:  0,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				v := b.checkOnTickStop(order, &tick)
				assert.Nil(t, v)
				assertNoErrorsGeneratedByBroker(t, b)
			}

			t.Log("Tick with price without fill")
			{
				order := newTestBrokerOrder(20.02, OrderBuy, 200, "id4")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.00,
					LastSize:  200,
					BidPrice:  20.99,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				v := b.checkOnTickStop(order, &tick)
				assert.Nil(t, v)
				assertNoErrorsGeneratedByBroker(t, b)
			}

			t.Log("Tick with exact order price")
			{
				order := newTestBrokerOrder(19.85, OrderBuy, 200, "id5")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 19.85,
					LastSize:  200,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
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
				order := newTestBrokerOrder(20.02, OrderBuy, 500, "id6")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.05,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

				order.BrokerExecQty = tick.LastSize
				order.BrokerState = PartialFilledOrder

				tick = marketdata.Tick{
					LastPrice: 20.07,
					LastSize:  900,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v = b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(300), v.(*OrderFillEvent).Qty)
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
				order := newTestBrokerOrder(20.02, OrderSell, 200, "ids1")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 19.98,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}

			t.Log("Tick with trade and with quote")
			{
				order := newTestBrokerOrder(20.02, OrderSell, 200, "ids2")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 19.95,
					LastSize:  200,
					BidPrice:  19.90,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
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
				order := newTestBrokerOrder(20.02, OrderSell, 200, "ids3")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: math.NaN(),
					LastSize:  0,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				v := b.checkOnTickStop(order, &tick)
				assert.Nil(t, v)
				assertNoErrorsGeneratedByBroker(t, b)

			}

			t.Log("Tick with price without fill")
			{
				order := newTestBrokerOrder(20.02, OrderSell, 200, "ids4")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.05,
					LastSize:  200,
					BidPrice:  20.99,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				v := b.checkOnTickStop(order, &tick)
				assert.Nil(t, v)
				assertNoErrorsGeneratedByBroker(t, b)
			}

			t.Log("Tick with exact order price")
			{
				order := newTestBrokerOrder(19.85, OrderSell, 200, "ids5")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 19.85,
					LastSize:  200,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
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
				order := newTestBrokerOrder(20.02, OrderSell, 500, "ids6")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 20.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v := b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(200), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}

				order.BrokerExecQty = tick.LastSize
				order.BrokerState = PartialFilledOrder

				tick = marketdata.Tick{
					LastPrice: 20.01,
					LastSize:  900,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				v = b.checkOnTickStop(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch v.(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
					assert.Equal(t, int64(300), v.(*OrderFillEvent).Qty)
					assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
				}
			}
		}
	}
}

func TestSimulatedBroker_checkOnTickMOO(t *testing.T) {
	b := newTestSimBrokerWorker()
	t.Log("LONG MOO order tests")
	{

		t.Log("Sim broker: test normal execution of MOO")
		{
			order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: true,
			}

			v := b.checkOnTickMOO(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
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
			order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id2")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: false,
			}

			v := b.checkOnTickMOO(order, &tick)
			assert.Nil(t, v)
			assertNoErrorsGeneratedByBroker(t, b)
		}
	}

	t.Log("SHORT MOO order tests")
	{

		t.Log("Sim broker: test normal execution of MOO")
		{
			order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids1")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: true,
			}

			v := b.checkOnTickMOO(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
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
			order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids2")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: false,
			}

			v := b.checkOnTickMOO(order, &tick)
			assert.Nil(t, v)
			assertNoErrorsGeneratedByBroker(t, b)
		}
	}
}

func TestSimulatedBroker_checkOnTickMOC(t *testing.T) {
	b := newTestSimBrokerWorker()
	t.Log("LONG MOC order tests")
	{

		t.Log("Sim broker: test normal execution of MOC")
		{
			order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: true,
			}

			v := b.checkOnTickMOC(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
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
			order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id2")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: false,
			}

			v := b.checkOnTickMOC(order, &tick)
			assert.Nil(t, v)
			assertNoErrorsGeneratedByBroker(t, b)
		}
	}

	t.Log("SHORT MOC order tests")
	{

		t.Log("Sim broker: test normal execution of MOC")
		{
			order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids1")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: true,
			}

			v := b.checkOnTickMOC(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
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
			order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids2")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: false,
			}

			v := b.checkOnTickMOC(order, &tick)
			assert.Nil(t, v)
			assertNoErrorsGeneratedByBroker(t, b)
		}
	}
}

func TestSimulatedBroker_checkOnTickLimitAuction(t *testing.T) {
	b := newTestSimBrokerWorker()
	b.marketOpenUntilTime = TimeOfDay{9, 33, 0}
	b.marketCloseUntilTime = TimeOfDay{16, 03, 0}
	t.Log("Sim broker: test auction orders LONG")
	{
		t.Log("Complete execution")
		{
			order := newTestBrokerOrder(15.87, OrderBuy, 200, "id1")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.80,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.checkOnTickLimitAuction(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)

			assert.NotNil(t, v)

			switch i := v[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, order.Qty, i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Tick without  execution")
		{
			order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.90,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),


				IsOpening: true,
			}

			v := b.checkOnTickLimitAuction(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)

			assert.NotNil(t, v)

			switch i := v[0].(type) {
			case *OrderCancelEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("strict and not strict limit orders execution")
		{
			t.Log("strict")
			{
				b.strictLimitOrders = true

				order := newTestBrokerOrder(15.87, OrderBuy, 200, "st10")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.checkOnTickLimitAuction(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)

				assert.NotNil(t, v)

				switch i := v[0].(type) {
				case *OrderCancelEvent:
					t.Log("OK! Got OrderCancelEvent as expected")
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}

			}

			t.Log("not strict")
			{
				b.strictLimitOrders = false

				order := newTestBrokerOrder(15.87, OrderBuy, 200, "st2")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.checkOnTickLimitAuction(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch i := v[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, order.Qty, i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}
			}
		}

		t.Log("Partial fill")
		{
			order := newTestBrokerOrder(15.87, OrderBuy, 1000, "id1z")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.80,
				LastSize:  389,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
				Datetime:  time.Date(2012, 1, 2, 9, 32, 0, 0, time.UTC),
			}

			v := b.checkOnTickLimitAuction(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
			assert.NotNil(t, v)

			switch i := v[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, int64(389), i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v, %v", i, i.getName())
			}

			switch i := v[1].(type) {
			case *OrderCancelEvent:
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v, %v", i, i.getName())
			}
		}
	}

	t.Log("Sim broker: test auction orders SHORT")
	{
		t.Log("Complete execution")
		{
			order := newTestBrokerOrder(15.87, OrderSell, 200, "ids1")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.90,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.checkOnTickLimitAuction(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
			assert.NotNil(t, v)

			switch i := v[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, order.Qty, i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("Tick without  execution")
		{
			order := newTestBrokerOrder(15.87, OrderSell, 200, "id2s")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.20,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.checkOnTickLimitAuction(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)
			assert.NotNil(t, v)

			switch i := v[0].(type) {
			case *OrderCancelEvent:
				t.Log("OK! Got OrderCancelEvent as expected")
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		t.Log("strict and not strict limit orders execution")
		{
			t.Log("strict")
			{
				b.strictLimitOrders = true
				order := newTestBrokerOrder(15.87, OrderSell, 200, "st1s")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.checkOnTickLimitAuction(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch i := v[0].(type) {
				case *OrderCancelEvent:
					t.Log("OK! Got OrderCancelEvent as expected")
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}
			}

			t.Log("not strict")
			{
				b.strictLimitOrders = false
				order := newTestBrokerOrder(15.87, OrderSell, 200, "st2s")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.checkOnTickLimitAuction(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
				assert.NotNil(t, v)

				switch i := v[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, order.Qty, i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
				}
			}

		}

		t.Log("Partial fill")
		{
			order := newTestBrokerOrder(15.87, OrderSell, 1000, "id1v")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.92,
				LastSize:  378,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.checkOnTickLimitAuction(order, &tick)
			assertNoErrorsGeneratedByBroker(t, b)

			assert.NotNil(t, v)

			switch i := v[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, int64(378), i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}

			switch i := v[1].(type) {
			case *OrderCancelEvent:
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
			}
		}
	}
}

func TestSimulatedBroker_checkOnTickLOC(t *testing.T) {
	b := newTestSimBrokerWorker()
	b.marketCloseUntilTime = TimeOfDay{16, 5, 0}
	t.Log("Sim broker: check LOC cancelation by time")
	{
		order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnClose
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 16, 7, 0, 0, time.UTC),
		}

		v := b.checkOnTickLOC(order, &tick)
		assertNoErrorsGeneratedByBroker(t, b)

		assert.NotNil(t, v)

		switch i := v[0].(type) {
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
			assert.Equal(t, order.Id, i.OrdId)
		default:
			t.Errorf("Error! Expected OrderCancelEvent. Got: %v", i)
		}
	}

	t.Log("Sim broker: check LOC hold by time. ")
	{
		order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnClose
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 16, 3, 0, 0, time.UTC),
		}

		v := b.checkOnTickLOC(order, &tick)
		assert.Nil(t, v)
		assertNoErrorsGeneratedByBroker(t, b)

	}
}

func TestSimulatedBroker_checkOnTickLOO(t *testing.T) {
	b := newTestSimBrokerWorker()
	b.marketOpenUntilTime = TimeOfDay{9, 35, 0}
	t.Log("Sim broker: check LOO cancelation by time")
	{
		order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnOpen
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 35, 1, 0, time.UTC),
		}

		v := b.checkOnTickLOO(order, &tick)
		assertNoErrorsGeneratedByBroker(t, b)
		assert.NotNil(t, v)

		switch i := v[0].(type) {
		case *OrderCancelEvent:
			t.Log("OK! Got OrderCancelEvent as expected")
			assert.Equal(t, order.Id, i.OrdId)
		default:
			t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
		}
	}

	t.Log("Sim broker: check LOO hold by time. ")
	{
		order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnOpen
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 31, 0, 0, time.UTC),
		}

		v := b.checkOnTickLOO(order, &tick)
		assert.Nil(t, v)
		assertNoErrorsGeneratedByBroker(t, b)
	}
}

func TestSimulatedBroker_OnTick(t *testing.T) {
	b := newTestSimBrokerWorker()
	b.strictLimitOrders = true
	b.marketOpenUntilTime = TimeOfDay{9, 35, 0}
	b.marketCloseUntilTime = TimeOfDay{16, 5, 15}

	putNewOrder := func(price float64, ordType OrderType, ordSide OrderSide, qty int64, id string) *simBrokerOrder {
		order := newTestBrokerOrder(price, ordSide, qty, id)
		order.Type = ordType
		assert.True(t, order.isValid())
		b.orders[order.Id] = order
		return order
	}

	onTick := func(t *marketdata.Tick) ([]error, []event) {
		wg := &sync.WaitGroup{}
		go func() {
			wg.Add(1)
			b.onTick(t)
			wg.Done()
		}()

		var errors []error
		var events []event
	LOOP:
		for {
			select {
			case e := <-b.ch.broker:
				events = append(events, e)
			case e := <-b.errChan:
				errors = append(errors, e)
			case b.ch.notifyBroker <- &BrokerNotifyEvent{}:
			case <-time.After(5 * time.Millisecond):
				break LOOP
			}
		}

		<-b.ch.brokerNotifier

		wg.Wait()

		return errors, events
	}

	order1 := putNewOrder(math.NaN(), MarketOrder, OrderBuy, 200, "1")
	order2 := putNewOrder(20.05, LimitOrder, OrderSell, 120, "2")
	order3 := putNewOrder(math.NaN(), MarketOnOpen, OrderSell, 300, "3")
	order4 := putNewOrder(10.03, LimitOnClose, OrderBuy, 200, "4")
	order5 := putNewOrder(50.08, StopOrder, OrderBuy, 90, "5")
	order6 := putNewOrder(19.08, LimitOnOpen, OrderBuy, 90, "6")
	order7 := putNewOrder(20.08, LimitOnClose, OrderSell, 90, "7")

	symbol := order1.Symbol
	fmt.Println(symbol)
	initalLen := len(b.orders)

	t.Log("Sim Broker: onTick. Reaction on broken tick. Error expected")
	{
		tick := marketdata.Tick{
			Symbol:    "Test",
			LastPrice: math.NaN(),
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}

		assert.False(t, b.tickIsValid(&tick))
		prevLen := len(b.orders)
		errors, events := onTick(&tick)

		assert.Len(t, b.orders, prevLen)
		assert.Len(t, events, 0)
		assert.Len(t, errors, 1)
		err := errors[0]
		assert.NotNil(t, err)
		assert.IsType(t, &ErrBrokenTick{}, err)
	}

	t.Log("Sim Broker: onTick. First tick - execute only market")
	{
		tick := marketdata.Tick{
			Symbol:    symbol,
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}

		errors, events := onTick(&tick)
		assert.Len(t, errors, 0)
		assert.Len(t, events, 1)

		assert.NotNil(t, events[0])

		switch i := events[0].(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, i.Price)
			assert.Equal(t, order1.Qty, i.Qty)
			assert.Equal(t, order1.Id, i.OrdId)
		default:
			t.Errorf("Error! Expected OrderFillEvent. Got: %v", i)
		}

		assert.Len(t, b.orders, initalLen)
		assert.Equal(t, FilledOrder, order1.BrokerState)

	}

	t.Log("Sim Broker: onTick. Second tick - execute nothing")
	{
		tick := marketdata.Tick{
			Symbol:    symbol,
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}

		errors, events := onTick(&tick)
		assert.Len(t, errors, 0)
		assert.Len(t, events, 0)
		assert.Len(t, b.orders, initalLen)
	}

	t.Log("Sim Broker: onTick. Third tick - execute only sell limit")
	{
		tick := marketdata.Tick{
			Symbol:    symbol,
			LastPrice: 20.06,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 9, 31, 1, 0, time.UTC),
		}

		errors, events := onTick(&tick)
		assert.Len(t, errors, 0)
		assert.Len(t, events, 1)

		v := events[0]
		assert.NotNil(t, v)

		switch i := v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, order2.Price, i.Price)
			assert.Equal(t, order2.Qty, i.Qty)
			assert.Equal(t, order2.Id, i.OrdId)
		default:
			t.Errorf("Error! Expected OrderFillEvent. Got: %v", i)
		}
		assert.Len(t, b.orders, initalLen)
		assert.Equal(t, FilledOrder, order2.BrokerState)

	}

	t.Log("Sim Broker: onTick. Forth tick - execute on open orders")
	{
		tick := marketdata.Tick{
			Symbol:    symbol,
			LastPrice: 15.88,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: true,
			Datetime:  time.Date(2010, 5, 5, 9, 30, 1, 0, time.UTC),
		}

		errors, events := onTick(&tick)
		assert.Len(t, errors, 0)
		assert.Len(t, events, 2)

		for _, v := range events {
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
		assert.Equal(t, FilledOrder, order3.BrokerState)
		assert.Equal(t, FilledOrder, order6.BrokerState)
		assert.Len(t, b.orders, initalLen)
	}

	t.Log("Sim Broker: onTick. Fifth tick - execute stop")
	{
		tick := marketdata.Tick{
			Symbol:    symbol,
			LastPrice: 50.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsOpening: false,
			Datetime:  time.Date(2010, 5, 5, 15, 30, 1, 0, time.UTC),
		}

		errors, events := onTick(&tick)
		assert.Len(t, errors, 0)
		assert.Len(t, events, 1)

		v := events[0]

		assert.NotNil(t, v)

		switch i := v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, i.Price)
			assert.Equal(t, order5.Qty, i.Qty)
			assert.Equal(t, order5.Id, i.OrdId)
		default:
			t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
		}

		assert.Len(t, b.orders, initalLen)
		assert.Equal(t, FilledOrder, order5.BrokerState)
	}

	t.Log("Sim Broker: onTick. Sixth tick - execute one on close and cancel another")
	{
		tick := marketdata.Tick{
			Symbol:    symbol,
			LastPrice: 10,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsClosing: true,
			Datetime:  time.Date(2010, 5, 5, 16, 01, 1, 0, time.UTC),
		}

		errors, events := onTick(&tick)
		assert.Len(t, errors, 0)
		assert.Len(t, events, 2)

		for _, v :=range events{
			assert.NotNil(t, v)
			switch i := v.(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, order4.Qty, i.Qty)
				assert.Equal(t, order4.Id, i.OrdId)
			case *OrderCancelEvent:
				t.Log("OK! Got OrderCancelEvent as expected")
				assert.Equal(t, order7.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
			}
		}

		assert.Len(t, b.orders, initalLen)
		assert.Equal(t, FilledOrder, order4.BrokerState)
		assert.Equal(t, CanceledOrder, order7.BrokerState)
	}

	t.Log("Sim Broker: onTick. React when we don't have confirmed orders")
	{

		tick := marketdata.Tick{
			Symbol:    symbol,
			LastPrice: 10,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			IsClosing: false,
			Datetime:  time.Date(2010, 5, 5, 16, 01, 1, 0, time.UTC),
		}

		errors, events := onTick(&tick)
		assert.Len(t, events, 0)
		assert.Len(t, errors, 0)
		assert.Len(t, b.orders, initalLen)

	}

	for _, o := range b.orders{
		assert.Equal(t, NewOrder, o.State)
	}

}
