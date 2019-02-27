package engine

import (
	"alex/marketdata"
	"github.com/stretchr/testify/assert"
	"math"
	"sync"
	"testing"
	"time"
)

/*import (
	"alex/marketdata"
	"github.com/stretchr/testify/assert"
	"math"
	"sync"
	"testing"
	"time"
)*/

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
		/*TODO надо модифицировать самого брокера. Убрать настройки в отдельный
		struct и мочить его в инит*/
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

func getFromSimBrokerWorkerError(ch chan error) error {
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

func putCancelRequestToWorkerAndGetError(w *simBrokerWorker, orderId string) error {
	wg := &sync.WaitGroup{}
	go func() {
		wg.Add(1)
		w.onCancelRequest(&OrderCancelRequestEvent{OrdId: orderId})
		wg.Done()
	}()

	v := getFromSimBrokerWorkerError(w.errChan)
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

/*
func assertNoEventsGeneratedByBroker(t *testing.T, b *simBrokerWorker) {
	select {
	case v, ok := <-b.mdChan:
		assert.False(t, ok)
		if ok {
			t.Errorf("ERROR! Expected no events. Found: %v", v)
		}
	default:
		t.Log("OK! Events chan is empty")
		break
	}
}



func getTestSimBrokerGeneratedEvent(b *simBrokerWorker) event {

	select {
	case v := <-b.mdChan:
		return v
	case <-time.After(1 * time.Second):
		return nil

	}
}

func getTestSimBrokerGeneratedErrors(b *simBrokerWorker) error {

	select {
	case v := <-b.errChan:
		return v
	case <-time.After(1 * time.Second):
		return nil

	}

}*/

func newTestBrokerOrder(price float64, side OrderSide, qty int, id string) *simBrokerOrder {
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
				b.orders[order.Id] = order

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
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
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
				b.orders[order.Id] = order

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
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
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
				b.orders[order.Id] = order

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
					assert.Equal(t, 100, v.(*OrderFillEvent).Qty)
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
				b.orders[order.Id] = order

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
					assert.Equal(t, 200, v.(*OrderFillEvent).Qty)
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
					assert.Equal(t, 300, v.(*OrderFillEvent).Qty)
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
				b.orders[order.Id] = order

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
				}

				v = b.checkOnTickMarket(order, &tick)
				assertNoErrorsGeneratedByBroker(t, b)
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
		t.Log("Sim broker: execute only when we got tick with prices")
		{
			order := newTestBrokerOrder(math.NaN(), OrderSell, 100, "Market7")
			order.Type = MarketOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

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
			b.orders[order.Id] = order

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
			b.orders[order.Id] = order

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

/*

func TestSimulatedBroker_checkOnTickLimit(t *testing.T) {
b := newTestSimBroker()

t.Log("Sim broker: test limit exectution for not limit order")
{
	order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "Wid1")
	order.Type = MarketOrder

	order.State = ConfirmedOrder
	assert.True(t, order.isValid())
	b.orders[order.Id] = order

	tick := marketdata.Tick{
		LastPrice: 20.00,
		LastSize:  200,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
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
	order := newTestBrokerOrder(20.01, OrderBuy, 200, "Wid10")

	order.State = FilledOrder
	order.ExecQty = 200
	assert.True(t, order.isValid())
	b.filledOrders[order.Id] = order

	tick := marketdata.Tick{
		LastPrice: 20.00,
		LastSize:  200,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
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
	order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "Wid2")

	assert.False(t, order.isValid())

	order.State = ConfirmedOrder

	b.orders[order.Id] = order

	tick := marketdata.Tick{
		LastPrice: 20.00,
		LastSize:  200,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
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
	order := newTestBrokerOrder(20, OrderBuy, 200, "Wid3")

	assert.True(t, order.isValid())

	order.State = ConfirmedOrder
	assert.True(t, order.isValid())
	b.orders[order.Id] = order

	//Case when tick has tag but don't have price
	tick := marketdata.Tick{
		LastPrice: math.NaN(),
		LastSize:  200,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
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
	order := newTestBrokerOrder(20, OrderBuy, 200, "Wid4")

	assert.True(t, order.isValid())

	order.State = newOrder
	assert.True(t, order.isValid())

	tick := marketdata.Tick{
		LastPrice: 19.88,
		LastSize:  200,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
	}

	b.checkOnTickLimit(order, &tick)
	assert.Equal(t, newOrder, order.State)

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
			order := newTestBrokerOrder(20.02, OrderBuy, 200, "id1")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)

		}

		t.Log("Partial order fill")
		{
			order := newTestBrokerOrder(20.13, OrderBuy, 200, "id2")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			t.Log("First tick without execution")
			{
				tick := marketdata.Tick{
					LastPrice: 20.15,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					
					
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
					
					
				}

				b.checkOnTickLimit(order, &tick)
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

				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)

			}

		}

	}

	t.Log("Sim broker: test strict Limit ordres")
	{
		b.strictLimitOrders = true

		order := newTestBrokerOrder(10.02, OrderBuy, 200, "id3")

		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		t.Log("Tick with order price but without fill")
		{

			tick := marketdata.Tick{
				LastPrice: 10.02,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
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
				
				
			}

			b.checkOnTickLimit(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

	}

	t.Log("Sim broker: test not strict Limit orders")
	{
		b.strictLimitOrders = false

		t.Log("Tick with order price with fill")
		{
			order := newTestBrokerOrder(10.15, OrderBuy, 200, "id4")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 10.15,
				LastSize:  500,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)

		}

		t.Log("Tick below order price")
		{
			order := newTestBrokerOrder(10.08, OrderBuy, 200, "id5")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 10.01,
				LastSize:  400,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
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

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.07,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)

		}

		t.Log("Partial order fill")
		{
			order := newTestBrokerOrder(20.13, OrderSell, 200, "ids2")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			t.Log("First tick without execution")
			{
				tick := marketdata.Tick{
					LastPrice: 20.11,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					
					
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
					
					
				}

				b.checkOnTickLimit(order, &tick)
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

				assert.Equal(t, ConfirmedOrder, order.State)
				assert.Len(t, b.confirmedOrders, 0)

			}

		}

	}

	t.Log("Sim broker: test strict Limit ordres")
	{
		b.strictLimitOrders = true

		order := newTestBrokerOrder(10.02, OrderSell, 200, "ids3")

		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		t.Log("Tick with order price but without fill")
		{

			tick := marketdata.Tick{
				LastPrice: 10.02,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
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
				
				
			}

			b.checkOnTickLimit(order, &tick)

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
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

	}

	t.Log("Sim broker: test not strict Limit orders")
	{
		b.strictLimitOrders = false

		t.Log("Tick with order price with fill")
		{
			order := newTestBrokerOrder(10.15, OrderSell, 200, "ids4")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 10.15,
				LastSize:  500,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)

		}

		t.Log("Tick above short order price")
		{
			order := newTestBrokerOrder(10.08, OrderSell, 200, "ids5")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 10.09,
				LastSize:  400,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("Tick with order price and partail fill")
		{
			order := newTestBrokerOrder(10.15, OrderSell, 200, "ids6")

			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 10.15,
				LastSize:  100,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)
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

			order.ExecQty = 100
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 1)

			tick = marketdata.Tick{
				LastPrice: 10.15,
				LastSize:  100,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickLimit(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

	}
}

}

func TestSimulatedBroker_checkOnTickStop(t *testing.T) {
b := newTestSimBroker()
t.Log("Sim broker: test stop order execution on tick")
{
	t.Log("LONG orders")
	{
		t.Log("Tick with trade but without quote")
		{
			order := newTestBrokerOrder(20.02, OrderBuy, 200, "id1")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.07,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickStop(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("Tick with trade and with quote")
		{
			order := newTestBrokerOrder(20.02, OrderBuy, 200, "id2")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.09,
				LastSize:  200,
				BidPrice:  20.08,
				AskPrice:  20.12,
				
				
			}

			b.checkOnTickStop(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("Tick without trade and with quote")
		{
			order := newTestBrokerOrder(20.02, OrderBuy, 200, "id3")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: math.NaN(),
				LastSize:  0,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  false,
				
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
			order := newTestBrokerOrder(20.02, OrderBuy, 200, "id4")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  20.99,
				AskPrice:  20.12,
				
				
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
			order := newTestBrokerOrder(19.85, OrderBuy, 200, "id5")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 19.85,
				LastSize:  200,
				BidPrice:  20.08,
				AskPrice:  20.12,
				
				
			}

			b.checkOnTickStop(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("Tick with trade and without quote and partial fills")
		{
			order := newTestBrokerOrder(20.02, OrderBuy, 500, "id6")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.05,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
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
				
				
			}

			b.checkOnTickStop(order, &tick)

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

			assert.Equal(t, PartialFilledOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}
	}

	t.Log("SHORT orders")
	{
		t.Log("Tick with trade but without quote")
		{
			order := newTestBrokerOrder(20.02, OrderSell, 200, "ids1")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 19.98,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
			}

			b.checkOnTickStop(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("Tick with trade and with quote")
		{
			order := newTestBrokerOrder(20.02, OrderSell, 200, "ids2")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 19.95,
				LastSize:  200,
				BidPrice:  19.90,
				AskPrice:  20.12,
				
				
			}

			b.checkOnTickStop(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("Tick without trade and with quote")
		{
			order := newTestBrokerOrder(20.02, OrderSell, 200, "ids3")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: math.NaN(),
				LastSize:  0,
				BidPrice:  20.08,
				AskPrice:  20.12,
				HasTrade:  false,
				
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
			order := newTestBrokerOrder(20.02, OrderSell, 200, "ids4")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.05,
				LastSize:  200,
				BidPrice:  20.99,
				AskPrice:  20.12,
				
				
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
			order := newTestBrokerOrder(19.85, OrderSell, 200, "ids5")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 19.85,
				LastSize:  200,
				BidPrice:  20.08,
				AskPrice:  20.12,
				
				
			}

			b.checkOnTickStop(order, &tick)

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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("Tick with trade and without quote and partial fills")
		{
			order := newTestBrokerOrder(20.02, OrderSell, 500, "ids6")
			order.Type = StopOrder
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 20.02,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
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
				
				
			}

			b.checkOnTickStop(order, &tick)

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

			assert.Equal(t, PartialFilledOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}
	}

}
}

func TestSimulatedBroker_checkOnTickMOO(t *testing.T) {
b := newTestSimBroker()
t.Log("LONG MOO order tests")
{

	t.Log("Sim broker: test normal execution of MOO")
	{
		order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
		order.Type = MarketOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
			IsOpening: true,
		}

		b.checkOnTickMOO(order, &tick)

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
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("Sim broker: tick that is not MOO")
	{
		order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id2")
		order.Type = MarketOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
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
		order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids1")
		order.Type = MarketOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
			IsOpening: true,
		}

		b.checkOnTickMOO(order, &tick)

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
		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("Sim broker: tick that is not MOO")
	{
		order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids2")
		order.Type = MarketOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
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
b := newTestSimBroker()
t.Log("LONG MOC order tests")
{

	t.Log("Sim broker: test normal execution of MOC")
	{
		order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
		order.Type = MarketOnClose
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
			IsClosing: true,
		}

		b.checkOnTickMOC(order, &tick)

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

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("Sim broker: tick that is not MOC")
	{
		order := newTestBrokerOrder(math.NaN(), OrderBuy, 200, "id2")
		order.Type = MarketOnClose
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
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
		order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids1")
		order.Type = MarketOnClose
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
			IsClosing: true,
		}

		b.checkOnTickMOC(order, &tick)

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

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("Sim broker: tick that is not MOC")
	{
		order := newTestBrokerOrder(math.NaN(), OrderSell, 200, "ids2")
		order.Type = MarketOnClose
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 20.09,
			LastSize:  2000,
			BidPrice:  20.08,
			AskPrice:  20.12,
			
			
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
b := newTestSimBroker()
b.marketOpenUntilTime = TimeOfDay{9, 33, 0}
b.marketCloseUntilTime = TimeOfDay{16, 03, 0}
t.Log("Sim broker: test auction orders LONG")
{
	t.Log("Complete execution")
	{
		order := newTestBrokerOrder(15.87, OrderBuy, 200, "id1")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.80,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			
			
			IsOpening: true,
		}

		b.checkOnTickLimitAuction(order, &tick)

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

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("Tick without  execution")
	{
		order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			
			
			IsOpening: true,
		}

		b.checkOnTickLimitAuction(order, &tick)
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

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("strict and not strict limit orders execution")
	{
		t.Log("strict")
		{
			b.strictLimitOrders = true
			order := newTestBrokerOrder(15.87, OrderBuy, 200, "st10")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.87,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("not strict")
		{
			b.strictLimitOrders = false
			order := newTestBrokerOrder(15.87, OrderBuy, 200, "st2")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.87,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

	}

	t.Log("Partial fill")
	{
		order := newTestBrokerOrder(15.87, OrderBuy, 1000, "id1z")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.80,
			LastSize:  389,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			
			
			IsOpening: true,
			Datetime:  time.Date(2012, 1, 2, 9, 32, 0, 0, time.UTC),
		}

		b.checkOnTickLimitAuction(order, &tick)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
			assert.Equal(t, 389, v.(*OrderFillEvent).Qty)
			assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

		default:
			t.Errorf("Error! Expected OrderFillEvent. Got: %v, %v", v, v.(event).getName())
		}

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)

		v = getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
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
		order := newTestBrokerOrder(15.87, OrderSell, 200, "ids1")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.90,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			
			
			IsOpening: true,
		}

		b.checkOnTickLimitAuction(order, &tick)
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

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("Tick without  execution")
	{
		order := newTestBrokerOrder(15.87, OrderSell, 200, "id2s")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.20,
			LastSize:  2000,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			
			
			IsOpening: true,
		}

		b.checkOnTickLimitAuction(order, &tick)
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

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 0)
	}

	t.Log("strict and not strict limit orders execution")
	{
		t.Log("strict")
		{
			b.strictLimitOrders = true
			order := newTestBrokerOrder(15.87, OrderSell, 200, "st1s")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.87,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
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

			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

		t.Log("not strict")
		{
			b.strictLimitOrders = false
			order := newTestBrokerOrder(15.87, OrderSell, 200, "st2s")
			order.Type = LimitOnOpen
			order.State = ConfirmedOrder
			assert.True(t, order.isValid())
			b.orders[order.Id] = order

			tick := marketdata.Tick{
				LastPrice: 15.87,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				
				
				IsOpening: true,
			}

			b.checkOnTickLimitAuction(order, &tick)
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
			assert.Equal(t, ConfirmedOrder, order.State)
			assert.Len(t, b.confirmedOrders, 0)
		}

	}

	t.Log("Partial fill")
	{
		order := newTestBrokerOrder(15.87, OrderSell, 1000, "id1v")
		order.Type = LimitOnOpen
		order.State = ConfirmedOrder
		assert.True(t, order.isValid())
		b.orders[order.Id] = order

		tick := marketdata.Tick{
			LastPrice: 15.92,
			LastSize:  378,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
			
			
			IsOpening: true,
		}

		b.checkOnTickLimitAuction(order, &tick)
		assertNoErrorsGeneratedByBroker(t, b)

		v := getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {
		case *OrderFillEvent:
			t.Log("OK! Got OrderFillEvent as expected")
			assert.Equal(t, tick.LastPrice, v.(*OrderFillEvent).Price)
			assert.Equal(t, 378, v.(*OrderFillEvent).Qty)
			assert.Equal(t, order.Id, v.(*OrderFillEvent).OrdId)

		default:
			t.Errorf("Error! Expected OrderFillEvent. Got: %v", v)
		}

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, b.confirmedOrders, 1)

		v = getTestSimBrokerGeneratedEvent(b)
		assert.NotNil(t, v)

		switch v.(type) {

		case *OrderCancelEvent:
			assert.Equal(t, order.Id, v.(*OrderCancelEvent).OrdId)
		default:
			t.Errorf("Error! Expected OrderCancelEvent. Got: %v", v)
		}
	}
}
}

func TestSimulatedBroker_checkOnTickLOC(t *testing.T) {
b := newTestSimBroker()
b.marketCloseUntilTime = TimeOfDay{16, 5, 0}
t.Log("Sim broker: check LOC cancelation by time")
{
	order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
	order.Type = LimitOnClose
	order.State = ConfirmedOrder
	assert.True(t, order.isValid())
	b.orders[order.Id] = order

	tick := marketdata.Tick{
		LastPrice: 15.90,
		LastSize:  2000,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
		IsOpening: false,
		Datetime:  time.Date(2010, 5, 5, 16, 7, 0, 0, time.UTC),
	}

	b.checkOnTickLOC(order, &tick)

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

	assert.Equal(t, ConfirmedOrder, order.State)
	assert.Len(t, b.confirmedOrders, 0)
}

t.Log("Sim broker: check LOC hold by time. ")
{
	order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
	order.Type = LimitOnClose
	order.State = ConfirmedOrder
	assert.True(t, order.isValid())
	b.orders[order.Id] = order

	tick := marketdata.Tick{
		LastPrice: 15.90,
		LastSize:  2000,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
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
b := newTestSimBroker()
b.marketOpenUntilTime = TimeOfDay{9, 35, 0}
t.Log("Sim broker: check LOO cancelation by time")
{
	order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
	order.Type = LimitOnOpen
	order.State = ConfirmedOrder
	assert.True(t, order.isValid())
	b.orders[order.Id] = order

	tick := marketdata.Tick{
		LastPrice: 15.90,
		LastSize:  2000,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
		IsOpening: false,
		Datetime:  time.Date(2010, 5, 5, 9, 35, 1, 0, time.UTC),
	}

	b.checkOnTickLOO(order, &tick)

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

	assert.Equal(t, ConfirmedOrder, order.State)
	assert.Len(t, b.confirmedOrders, 0)
}

t.Log("Sim broker: check LOO hold by time. ")
{
	order := newTestBrokerOrder(15.87, OrderBuy, 200, "id2")
	order.Type = LimitOnOpen
	order.State = ConfirmedOrder
	assert.True(t, order.isValid())
	b.orders[order.Id] = order

	tick := marketdata.Tick{
		LastPrice: 15.90,
		LastSize:  2000,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
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
b := newTestSimBroker()
b.strictLimitOrders = true
b.marketOpenUntilTime = TimeOfDay{9, 35, 0}
b.marketCloseUntilTime = TimeOfDay{16, 5, 15}

putNewOrder := func(price float64, ordType OrderType, ordSide OrderSide, qty int, id string) *Order {
	order := newTestBrokerOrder(price, ordSide, qty, id)
	order.State = ConfirmedOrder
	order.Type = ordType
	assert.True(t, order.isValid())
	prevLen := len(b.confirmedOrders)
	b.orders[order.Id] = order
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

symbol := order1.Symbol

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

	prevLen := len(b.confirmedOrders)
	b.onTick(&tick)
	assert.Len(t, b.confirmedOrders, prevLen)
	assertNoEventsGeneratedByBroker(t, b)

	err := getTestSimBrokerGeneratedErrors(b)
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

	prevLen := len(b.confirmedOrders)
	prevLenFills := len(b.filledOrders)
	b.onTick(&tick)

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

	assert.Len(t, b.confirmedOrders, prevLen-1)
	assert.Len(t, b.filledOrders, prevLenFills+1)

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
	prevLen := len(b.confirmedOrders)
	b.onTick(&tick)
	assert.Len(t, b.confirmedOrders, prevLen)
	assertNoErrorsGeneratedByBroker(t, b)
	assertNoEventsGeneratedByBroker(t, b)

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
	prevLen := len(b.confirmedOrders)
	prevLenFills := len(b.filledOrders)
	b.onTick(&tick)

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
	assert.Len(t, b.confirmedOrders, prevLen-1)
	assert.Len(t, b.filledOrders, prevLenFills+1)

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
	prevLen := len(b.confirmedOrders)
	prevLenFills := len(b.filledOrders)
	b.onTick(&tick)

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

	time.Sleep(2 * time.Millisecond) //todo проверить. Это дб синхронно как то
	assert.Len(t, b.confirmedOrders, prevLen-2)
	assert.Len(t, b.filledOrders, prevLenFills+2)

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
	prevLen := len(b.confirmedOrders)
	prevLenFills := len(b.filledOrders)
	b.onTick(&tick)

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

	assert.Len(t, b.confirmedOrders, prevLen-1)
	assert.Len(t, b.filledOrders, prevLenFills+1)

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
	prevLen := len(b.confirmedOrders)
	prevLenFills := len(b.filledOrders)
	b.onTick(&tick)

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

	time.Sleep(2 * time.Millisecond) //todo проверить. Это дб синхронно как то
	assert.Len(t, b.confirmedOrders, prevLen-2)
	assert.Len(t, b.filledOrders, prevLenFills+1)
	assert.Len(t, b.canceledOrders, 1)

}

t.Log("Sim Broker: onTick. React when we don't have confirmed orders")
{
	assert.Len(t, b.confirmedOrders, 0)
	tick := marketdata.Tick{
		Symbol:    symbol,
		LastPrice: 10,
		LastSize:  2000,
		BidPrice:  math.NaN(),
		AskPrice:  math.NaN(),
		
		
		IsClosing: false,
		Datetime:  time.Date(2010, 5, 5, 16, 01, 1, 0, time.UTC),
	}

	b.onTick(&tick)

	assertNoErrorsGeneratedByBroker(t, b)
	assertNoEventsGeneratedByBroker(t, b)

}

}*/
