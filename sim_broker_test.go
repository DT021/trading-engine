package engine

import (
	"alex/marketdata"
	"github.com/stretchr/testify/assert"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func newTestSimBroker() *SimBroker {
	b := SimBroker{delay: 1000}
	b.checkExecutionsOnTicks = true
	errChan := make(chan error)
	events := make(chan event)
	b.Init(errChan, events, []*Instrument{&Instrument{}})
	return &b
}

func TestEventArray_Sort(t *testing.T) {
	var events eventArray
	i := 0
	for i < 20 {
		e := NewTickEvent{BaseEvent: be(time.Now().Add(time.Duration(rand.Int())*time.Second), newTestInstrument())}
		events = append(events, &e)
		i ++
	}
	sorted := true

	for i, e := range events {
		if i == 0 {
			continue
		}
		if e.getTime().Before(events[i-1].getTime()) {
			sorted = false
		}

	}

	assert.False(t, sorted)
	events.sort()
	for i, e := range events {
		if i == 0 {
			continue
		}

		assert.False(t, e.getTime().Before(events[i-1].getTime()))
	}
}

func newTestSimBrokerWorker() *simBrokerWorker {
	w := simBrokerWorker{
		symbol:            newTestInstrument(),
		errChan:           make(chan error),
		events:            make(chan event),
		delay:             100,
		strictLimitOrders: false,
		mpMutext:          &sync.RWMutex{},
		orders:            make(map[string]*simBrokerOrder),
		waitGroup:         &sync.WaitGroup{},
	}

	return &w
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
			t.Fatalf("Fatal.Expected OrderConfirmationEvent. Got %+v", v)
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

	w.onNewOrder(&NewOrderEvent{
		LinkedOrder: order,
		BaseEvent:   be(order.Time, order.Ticker)})

	if len(w.generatedEvents) == 0 {
		return nil
	}

	return w.generatedEvents[len(w.generatedEvents)-1]
}

func putCancelRequestToWorkerAndGetBrokerEvent(w *simBrokerWorker, orderId string) event {
	w.onCancelRequest(&OrderCancelRequestEvent{OrdId: orderId})
	if len(w.generatedEvents) == 0 {
		return nil
	}

	return w.generatedEvents[len(w.generatedEvents)-1]
}

func putReplaceRequestToWorkerAndGetBrokerEvent(w *simBrokerWorker, orderId string, newPrice float64) event {
	w.onReplaceRequest(&OrderReplaceRequestEvent{OrdId: orderId, NewPrice: newPrice})
	if len(w.generatedEvents) == 0 {
		return nil
	}
	return w.generatedEvents[len(w.generatedEvents)-1]
}

func newTestInstrument() *Instrument {
	inst := Instrument{
		Symbol:  "Test",
		LotSize: 100,
		MinTick: 0.01,
		Exchange: Exchange{
			Name:            "TestExchange",
			MarketCloseTime: TimeOfDay{16, 0, 0},
			MarketOpenTime:  TimeOfDay{9, 30, 0},
		},
	}

	return &inst
}

func putOrderAndFillOnTick(b *simBrokerWorker, order *simBrokerOrder, tickRaw *marketdata.Tick) ([]event, []error) {
	tick := Tick{
		Tick:   tickRaw,
		Ticker: newTestInstrument(),
	}
	te := NewTickEvent{
		be(tick.Datetime, tick.Ticker),
		&tick,
	}

	b.orders[order.Id] = order
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		b.onTick(&te)
		wg.Done()
	}()
	var events []event
	var errors []error
eventloop:
	for {
		select {
		case e := <-b.events:
			events = append(events, e)
		case <-time.After(30 * time.Millisecond):
			break eventloop
		}
	}

errorsLoop:
	for {
		select {
		case e := <-b.errChan:
			errors = append(errors, e)
		case <-time.After(30 * time.Millisecond):
			break errorsLoop
		}
	}
	wg.Wait()
	delete(b.orders, order.Id)
	return events, errors

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
			t.Fatalf("Fatal.Expected OrderCancelEvent. Got %+v", v)
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

func newTestGtcBrokerOrder(price float64, side OrderSide, qty int64, id string) *simBrokerOrder {
	ord := newTestOrder(price, side, qty, id)
	o := simBrokerOrder{
		Order:        ord,
		BrokerState:  ConfirmedOrder,
		BrokerPrice:  price,
		StateUpdTime: newTestOrderTime(),
	}
	return &o
}

func newTestOpgBrokerOrder(price float64, side OrderSide, qty int64, id string) *simBrokerOrder {
	ord := newTestOrder(price, side, qty, id)
	ord.Tif = AuctionTIF
	o := simBrokerOrder{
		Order:        ord,
		BrokerState:  ConfirmedOrder,
		BrokerPrice:  price,
		StateUpdTime: newTestOpgOrderTime(),
	}
	return &o
}

func newTestDayBrokerOrder(price float64, side OrderSide, qty int64, id string) *simBrokerOrder {
	ord := newTestOrder(price, side, qty, id)
	ord.Tif = DayTIF
	o := simBrokerOrder{
		Order:        ord,
		BrokerState:  ConfirmedOrder,
		BrokerPrice:  price,
		StateUpdTime: newTestOrderTime(),
	}
	return &o
}

func newTestOpgOrderTime() time.Time {
	return time.Date(2010, 2, 2, 15, 10, 10, 15, time.UTC)
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

func TestSimulatedBroker_fillMarketOnTick(t *testing.T) {
	b := newTestSimBrokerWorker()

	t.Log("Case when sim broker has only trades feed")
	{
		t.Log("Sim broker: test normal market order execution on tick")
		{
			t.Log("Check execution when we have only last trade price")
			{

				order := newTestGtcBrokerOrder(math.NaN(), OrderSell, 100, "Market1")
				order.Type = MarketOrder
				order.BrokerState = ConfirmedOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 2),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				assert.True(t, tick.IsValid())

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)
				if len(events) > 0 {
					v := events[0]
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
	}

	t.Log("Case when sim broker has both quotes and trades feed. ")
	{

		t.Log("Sim broker: normal execution on tick with quotes")
		{
			t.Log("SHORT ORDER full execution")
			{

				order := newTestGtcBrokerOrder(math.NaN(), OrderSell, 100, "Market3")
				order.Type = MarketOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 2),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.95,
					AskPrice:  20.05,
					BidSize:   200,
					AskSize:   300,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)
				v := events[0]

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

				order := newTestGtcBrokerOrder(math.NaN(), OrderBuy, 100, "Market4")
				order.Type = MarketOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 2),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.95,
					AskPrice:  21.05,
					BidSize:   200,
					AskSize:   300,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)
				v := events[0]
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

				order := newTestGtcBrokerOrder(math.NaN(), OrderSell, 500, "Market5")
				order.Type = MarketOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 2),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.01,
					AskPrice:  20.05,
					BidSize:   200,
					AskSize:   300,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)
				v := events[0]
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

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, PartialFilledOrder, order.BrokerState)

				//New tick - fill rest of the order. New price*************************

				tick = marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.98,
					AskPrice:  20.05,
					BidSize:   800,
					AskSize:   300,
				}

				events, errors = putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)
				v = events[0]
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

				assert.Equal(t, int64(500), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("LONG ORDER partial execution")
			{

				order := newTestGtcBrokerOrder(math.NaN(), OrderBuy, 900, "Market6")
				order.Type = MarketOrder

				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  20.01,
					AskPrice:  20.09,
					BidSize:   200,
					AskSize:   600,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)
				v := events[0]
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

				assert.Equal(t, int64(600), order.BrokerExecQty)
				assert.Equal(t, PartialFilledOrder, order.BrokerState)

				//New tick - fill rest of the order. New price*************************
				tick = marketdata.Tick{
					Datetime:  time.Now().Add(time.Second * 4),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  200,
					BidPrice:  19.98,
					AskPrice:  20.05,
					BidSize:   800,
					AskSize:   300,
				}

				events, errors = putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)
				v = events[0]
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

				assert.Equal(t, int64(900), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

		}
		t.Log("Sim broker: execute only when we got tick with prices")
		{
			order := newTestGtcBrokerOrder(math.NaN(), OrderSell, 100, "Market7")
			order.Type = MarketOrder
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Second * 2),
				Symbol:    "Test",
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				BidSize:   0,
				AskSize:   0,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)
			v := events[0]
			assert.NotNil(t, v)

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
			order := newTestGtcBrokerOrder(20.0, OrderSell, 100, "Market7")
			order.Type = MarketOrder
			assert.False(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Second * 3),
				Symbol:    "Test",
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  19.95,
				AskPrice:  20.01,
				BidSize:   100,
				AskSize:   200,
			}

			assert.True(t, tick.IsValid())

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 1)

			assert.IsType(t, &ErrInvalidOrder{}, errors[0])

		}

	}

	t.Log("Day orders")
	{
		t.Log("Normal execution")
		{
			order := newTestDayBrokerOrder(math.NaN(), OrderSell, 100, "Market1")
			order.Type = MarketOrder
			order.BrokerState = ConfirmedOrder

			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Second * 2),
				Symbol:    "Test",
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			assert.True(t, tick.IsValid())

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)
			if len(events) > 0 {
				v := events[0]
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

		t.Log("Cancel because of TIF expiration")
		{
			order := newTestDayBrokerOrder(math.NaN(), OrderSell, 100, "Market1")
			order.Type = MarketOrder
			order.BrokerState = ConfirmedOrder

			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Hour * 25),
				Symbol:    "Test",
				LastPrice: 20.01,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			assert.True(t, tick.IsValid())

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)
			if len(events) > 0 {
				v := events[0]
				assert.NotNil(t, v)

				switch i := v.(type) {
				case *OrderCancelEvent:
					t.Log("OK! Got OrderCancelEvent as expected")
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderCancelEvent. Got: %v", i)
				}
			}
		}
	}
}

func TestSimulatedBroker_fillLimitOnTick(t *testing.T) {
	b := newTestSimBrokerWorker()

	t.Log("Sim broker: test limit execution for already filled order")
	{
		order := newTestGtcBrokerOrder(20.01, OrderBuy, 200, "Wid10")

		order.BrokerState = FilledOrder
		order.BrokerExecQty = 200
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			Datetime:  newTestOrderTime().Add(time.Second * 3),
			Symbol:    "Test",
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		events, errors := putOrderAndFillOnTick(b, order, &tick)
		assert.Len(t, events, 0)
		assert.Len(t, errors, 0)

	}

	t.Log("Sim broker: test limit execution for not valid order")
	{
		order := newTestGtcBrokerOrder(math.NaN(), OrderBuy, 200, "Wid2")
		assert.False(t, order.isValid())
		order.BrokerState = ConfirmedOrder

		tick := marketdata.Tick{
			Datetime:  newTestOrderTime().Add(time.Second * 3),
			Symbol:    "Test",
			LastPrice: 20.00,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		events, errors := putOrderAndFillOnTick(b, order, &tick)
		assert.Len(t, events, 0)
		assert.Len(t, errors, 1)
		assert.IsType(t, &ErrInvalidOrder{}, errors[0])
	}

	t.Log("Sim broker: test limit execution for tick without trade")
	{
		order := newTestGtcBrokerOrder(20, OrderBuy, 200, "Wid3")
		assert.True(t, order.isValid())

		//Case when tick has tag but don't have price
		tick := marketdata.Tick{
			Datetime:  newTestOrderTime().Add(time.Second * 3),
			Symbol:    "Test",
			LastPrice: math.NaN(),
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		assert.False(t, tick.IsValid())

		events, errors := putOrderAndFillOnTick(b, order, &tick)
		assert.Len(t, events, 0)
		assert.Len(t, errors, 1)
		assert.IsType(t, &ErrBrokenTick{}, errors[0])
	}

	t.Log("Sim broker: test limit execution for not confirmed order. No events and errors expected")
	{
		order := newTestGtcBrokerOrder(20, OrderBuy, 200, "Wid4")
		assert.True(t, order.isValid())
		order.BrokerState = NewOrder
		assert.True(t, order.isValid())

		tick := marketdata.Tick{
			Datetime:  newTestOrderTime().Add(time.Second * 3),
			Symbol:    "Test",
			LastPrice: 19.88,
			LastSize:  200,
			BidPrice:  math.NaN(),
			AskPrice:  math.NaN(),
		}

		events, errors := putOrderAndFillOnTick(b, order, &tick)
		assert.Len(t, events, 0)
		assert.Len(t, errors, 0)
	}

	t.Log("Sim broker: TEST LONG ORDERS")
	{

		t.Log("Sim broker: test limit order execution on tick")
		{
			t.Log("Normal execution")
			{
				order := newTestGtcBrokerOrder(20.02, OrderBuy, 200, "id1")
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Hour * 35),
					Symbol:    "Test",
					LastPrice: 20.00,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				assert.True(t, tick.IsValid())

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Partial order fill")
			{
				order := newTestGtcBrokerOrder(20.13, OrderBuy, 200, "id2")
				assert.True(t, order.isValid())

				t.Log("First tick without execution")
				{
					tick := marketdata.Tick{
						Datetime:  newTestOrderTime().Add(time.Second * 3),
						Symbol:    "Test",
						LastPrice: 20.15,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					events, errors := putOrderAndFillOnTick(b, order, &tick)
					assert.Len(t, events, 0)
					assert.Len(t, errors, 0)

					assert.Equal(t, int64(0), order.BrokerExecQty)
					assert.Equal(t, ConfirmedOrder, order.BrokerState)

				}

				t.Log("First fill")
				{
					tick := marketdata.Tick{
						Datetime:  newTestOrderTime().Add(time.Second * 3),
						Symbol:    "Test",
						LastPrice: 20.12,
						LastSize:  100,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					events, errors := putOrderAndFillOnTick(b, order, &tick)
					assert.Len(t, events, 1)
					assert.Len(t, errors, 0)

					switch i := events[0].(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, i.Price)
						assert.Equal(t, int64(100), i.Qty)
						assert.Equal(t, order.Id, i.OrdId)
					default:
						t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
					}

					assert.Equal(t, int64(100), order.BrokerExecQty)
					assert.Equal(t, PartialFilledOrder, order.BrokerState)

				}

				t.Log("Complete fill")
				{
					tick := marketdata.Tick{
						Datetime:  newTestOrderTime().Add(time.Second * 3),
						Symbol:    "Test",
						LastPrice: 20.12,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					events, errors := putOrderAndFillOnTick(b, order, &tick)
					assert.Len(t, events, 1)
					assert.Len(t, errors, 0)

					switch i := events[0].(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, i.Price)
						assert.Equal(t, int64(100), i.Qty)
						assert.Equal(t, order.Id, i.OrdId)
					default:
						t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
					}

					assert.Equal(t, int64(200), order.BrokerExecQty)
					assert.Equal(t, FilledOrder, order.BrokerState)
				}
			}
		}

		t.Log("Sim broker: test strict limit orders execution")
		{
			b.strictLimitOrders = true

			order := newTestGtcBrokerOrder(10.02, OrderBuy, 200, "id3")
			assert.True(t, order.isValid())

			t.Log("Tick with order price but without fill")
			{
				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 0)
				assert.Len(t, errors, 0)
			}

			t.Log("Tick below order price")
			{
				tick := marketdata.Tick{
					Datetime: newTestOrderTime().Add(time.Second * 3),
					Symbol:   "Test", LastPrice: 10.01,
					LastSize: 400,
					BidPrice: math.NaN(),
					AskPrice: math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

		}

		t.Log("Sim broker: test not strict Limit orders")
		{
			b.strictLimitOrders = false

			t.Log("Tick with order price with fill")
			{
				order := newTestGtcBrokerOrder(10.15, OrderBuy, 200, "id4")
				assert.True(t, order.isValid())
				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.15,
					LastSize:  500,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Tick below order price")
			{
				order := newTestGtcBrokerOrder(10.08, OrderBuy, 200, "id5")
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.01,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)

			}
		}
	}

	t.Log("Sim broker: TEST SHORT ORDERS")
	{
		t.Log("Sim broker: test limit order execution on tick")
		{
			t.Log("Normal execution")
			{
				order := newTestGtcBrokerOrder(20.02, OrderSell, 200, "ids1")

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.07,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Partial order fill")
			{
				order := newTestGtcBrokerOrder(20.13, OrderSell, 200, "ids2")

				t.Log("First tick without execution")
				{
					tick := marketdata.Tick{
						Datetime:  newTestOrderTime().Add(time.Second * 3),
						Symbol:    "Test",
						LastPrice: 20.11,
						LastSize:  200,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					events, errors := putOrderAndFillOnTick(b, order, &tick)
					assert.Len(t, events, 0)
					assert.Len(t, errors, 0)
				}

				t.Log("First fill")
				{
					tick := marketdata.Tick{
						Datetime:  newTestOrderTime().Add(time.Second * 3),
						Symbol:    "Test",
						LastPrice: 20.15,
						LastSize:  100,
						BidPrice:  math.NaN(),
						AskPrice:  math.NaN(),
					}

					events, errors := putOrderAndFillOnTick(b, order, &tick)
					assert.Len(t, events, 1)
					assert.Len(t, errors, 0)

					switch i := events[0].(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, i.Price)
						assert.Equal(t, int64(100), i.Qty)
						assert.Equal(t, order.Id, i.OrdId)
					default:
						t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
					}

					assert.Equal(t, int64(100), order.BrokerExecQty)
					assert.Equal(t, PartialFilledOrder, order.BrokerState)
				}

				t.Log("Complete fill")
				{
					tick := marketdata.Tick{
						Datetime: newTestOrderTime().Add(time.Second * 3),
						Symbol:   "Test", LastPrice: 20.18,
						LastSize: 200,
						BidPrice: math.NaN(),
						AskPrice: math.NaN(),
					}

					events, errors := putOrderAndFillOnTick(b, order, &tick)
					assert.Len(t, events, 1)
					assert.Len(t, errors, 0)

					switch i := events[0].(type) {
					case *OrderFillEvent:
						t.Log("OK! Got OrderFillEvent as expected")
						assert.Equal(t, order.Price, i.Price)
						assert.Equal(t, int64(100), i.Qty)
						assert.Equal(t, order.Id, i.OrdId)
					default:
						t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
					}

					assert.Equal(t, int64(200), order.BrokerExecQty)
					assert.Equal(t, FilledOrder, order.BrokerState)
				}
			}
		}

		t.Log("Sim broker: test strict limit orders execution")
		{
			b.strictLimitOrders = true

			order := newTestGtcBrokerOrder(10.02, OrderSell, 200, "ids3")

			t.Log("Tick with order price but without fill")
			{

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 0)
				assert.Len(t, errors, 0)

				assert.Equal(t, int64(0), order.BrokerExecQty)
				assert.Equal(t, ConfirmedOrder, order.BrokerState)
			}

			t.Log("Tick above short order price")
			{
				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.04,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}
		}

		t.Log("Sim broker: test not strict Limit orders")
		{
			b.strictLimitOrders = false

			t.Log("Tick with order price with fill")
			{
				order := newTestGtcBrokerOrder(10.15, OrderSell, 200, "ids4")
				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.15,
					LastSize:  500,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}
				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Tick above short order price")
			{
				order := newTestGtcBrokerOrder(10.08, OrderSell, 200, "ids5")
				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.09,
					LastSize:  400,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Tick with order price and partial fill")
			{
				order := newTestGtcBrokerOrder(10.15, OrderSell, 200, "ids6")
				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 10.15,
					LastSize:  100,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(100), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(100), order.BrokerExecQty)
				assert.Equal(t, PartialFilledOrder, order.BrokerState)

				tick = marketdata.Tick{
					Datetime:  time.Now().Add(time.Second * 4),
					Symbol:    "Test",
					LastPrice: 10.15,
					LastSize:  100,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors = putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, order.Price, i.Price)
					assert.Equal(t, int64(100), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}
		}
	}

	t.Log("Sim broker: test Day orders")
	{
		t.Log("Normal execution")
		{
			order := newTestDayBrokerOrder(20.02, OrderBuy, 200, "id1")
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Hour * 1),
				Symbol:    "Test",
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			assert.True(t, tick.IsValid())

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, order.Price, i.Price)
				assert.Equal(t, int64(200), i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
			}

			assert.Equal(t, int64(200), order.BrokerExecQty)
			assert.Equal(t, FilledOrder, order.BrokerState)
		}

		t.Log("Next day tick. Cancel expected")
		{
			order := newTestDayBrokerOrder(20.02, OrderBuy, 200, "id1")
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Hour * 26),
				Symbol:    "Test",
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			assert.True(t, tick.IsValid())

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderCancelEvent:
				t.Log("OK! Got OrderCancelEvent as expected")
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderCancelEvent. Got: %+v", i)
			}

			assert.Equal(t, int64(0), order.BrokerExecQty)
			assert.Equal(t, CanceledOrder, order.BrokerState)
		}
	}
}

func TestSimulatedBroker_fillStopOnTick(t *testing.T) {
	b := newTestSimBrokerWorker()
	t.Log("Sim broker: test stop order execution on tick")
	{

		t.Log("Sim broker: test stop order execution for already filled order")
		{
			order := newTestGtcBrokerOrder(20.01, OrderBuy, 200, "Wid10")
			order.Type = StopOrder
			order.BrokerState = FilledOrder
			order.BrokerExecQty = 200
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Second * 3),
				Symbol:    "Test",
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 0)

		}

		t.Log("Sim broker: test stop execution for not valid order")
		{
			order := newTestGtcBrokerOrder(math.NaN(), OrderBuy, 200, "Wid2")
			order.Type = StopOrder
			assert.False(t, order.isValid())
			order.BrokerState = ConfirmedOrder

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Second * 3),
				Symbol:    "Test",
				LastPrice: 20.00,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 1)
			assert.IsType(t, &ErrInvalidOrder{}, errors[0])
		}

		t.Log("Sim broker: test stop execution for tick without trade")
		{
			order := newTestGtcBrokerOrder(20, OrderBuy, 200, "Wid3")
			order.Type = StopOrder
			assert.True(t, order.isValid())

			//Case when tick has tag but don't have price
			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Second * 3),
				Symbol:    "Test",
				LastPrice: math.NaN(),
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 1)
			assert.IsType(t, &ErrBrokenTick{}, errors[0])
		}

		t.Log("Sim broker: test stop execution for not confirmed order")
		{
			order := newTestGtcBrokerOrder(20, OrderBuy, 200, "Wid4")
			order.Type = StopOrder
			assert.True(t, order.isValid())
			order.BrokerState = NewOrder
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Datetime:  newTestOrderTime().Add(time.Second * 3),
				Symbol:    "Test",
				LastPrice: 19.88,
				LastSize:  200,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 0)
		}

		t.Log("LONG orders")
		{
			t.Log("Tick with trade but without quote")
			{
				order := newTestGtcBrokerOrder(20.02, OrderBuy, 200, "id1")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.07,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)

			}

			t.Log("Tick with trade and with quote")
			{
				order := newTestGtcBrokerOrder(20.02, OrderBuy, 200, "id2")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.09,
					LastSize:  200,
					BidPrice:  20.08,
					BidSize:   200,
					AskSize:   200,
					AskPrice:  20.12,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.AskPrice, i.Price)
					assert.Equal(t, order.Qty, i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, order.Qty, order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)

			}

			t.Log("Tick without trade and with quote")
			{
				order := newTestGtcBrokerOrder(20.02, OrderBuy, 200, "id3")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: math.NaN(),
					LastSize:  0,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 0)
				assert.Len(t, errors, 0)
			}

			t.Log("Tick with price without fill")
			{
				order := newTestGtcBrokerOrder(20.02, OrderBuy, 200, "id4")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.00,
					LastSize:  200,
					BidPrice:  20.99,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 0)
				assert.Len(t, errors, 0)
			}

			t.Log("Tick with exact order price")
			{
				order := newTestGtcBrokerOrder(19.85, OrderBuy, 200, "id5")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 19.85,
					LastSize:  200,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.AskPrice, i.Price)
					assert.Equal(t, order.Qty, i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, order.Qty, order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Tick with trade and without quote and partial fills")
			{
				order := newTestGtcBrokerOrder(20.02, OrderBuy, 500, "id6")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.05,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, PartialFilledOrder, order.BrokerState)

				tick = marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.07,
					LastSize:  900,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors = putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, int64(300), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, order.Qty, order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}
		}

		t.Log("SHORT orders")
		{
			t.Log("Tick with trade but without quote")
			{
				order := newTestGtcBrokerOrder(20.02, OrderSell, 200, "ids1")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 19.98,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, order.Qty, order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Tick with trade and with quote")
			{
				order := newTestGtcBrokerOrder(20.02, OrderSell, 200, "ids2")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 19.95,
					LastSize:  200,
					BidPrice:  19.90,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.BidPrice, i.Price)
					assert.Equal(t, order.Qty, i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, order.Qty, order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)

			}

			t.Log("Tick without trade and with quote")
			{
				order := newTestGtcBrokerOrder(20.02, OrderSell, 200, "ids3")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: math.NaN(),
					LastSize:  0,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 0)
				assert.Len(t, errors, 0)

			}

			t.Log("Tick with price without fill")
			{
				order := newTestGtcBrokerOrder(20.02, OrderSell, 200, "ids4")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.05,
					LastSize:  200,
					BidPrice:  20.99,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 0)
				assert.Len(t, errors, 0)
			}

			t.Log("Tick with exact order price")
			{
				order := newTestGtcBrokerOrder(19.85, OrderSell, 200, "ids5")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 19.85,
					LastSize:  200,
					BidPrice:  20.08,
					AskPrice:  20.12,
					BidSize:   200,
					AskSize:   200,
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.BidPrice, i.Price)
					assert.Equal(t, order.Qty, i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, order.Qty, order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}

			t.Log("Tick with trade and without quote and partial fills")
			{
				order := newTestGtcBrokerOrder(20.02, OrderSell, 500, "ids6")
				order.Type = StopOrder
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.02,
					LastSize:  200,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors := putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, int64(200), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}

				assert.Equal(t, int64(200), order.BrokerExecQty)
				assert.Equal(t, PartialFilledOrder, order.BrokerState)

				tick = marketdata.Tick{
					Datetime:  newTestOrderTime().Add(time.Second * 3),
					Symbol:    "Test",
					LastPrice: 20.01,
					LastSize:  900,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
				}

				events, errors = putOrderAndFillOnTick(b, order, &tick)
				assert.Len(t, events, 1)
				assert.Len(t, errors, 0)

				switch i := events[0].(type) {
				case *OrderFillEvent:
					t.Log("OK! Got OrderFillEvent as expected")
					assert.Equal(t, tick.LastPrice, i.Price)
					assert.Equal(t, int64(300), i.Qty)
					assert.Equal(t, order.Id, i.OrdId)
				default:
					t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
				}
				assert.Equal(t, order.Qty, order.BrokerExecQty)
				assert.Equal(t, FilledOrder, order.BrokerState)
			}
		}
	}
}

func TestSimulatedBroker_fillMooOnTick(t *testing.T) {
	b := newTestSimBrokerWorker()
	t.Log("LONG MOO order tests")
	{

		t.Log("Sim broker: test normal execution of MOO")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: true,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, order.Qty, i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
			}

			assert.Equal(t, order.Qty, order.BrokerExecQty)
			assert.Equal(t, FilledOrder, order.BrokerState)

		}

		t.Log("Sim broker: tick that is not MOO")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderBuy, 200, "id2")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: false,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 0)
			assert.Equal(t, int64(0), order.BrokerExecQty)
			assert.Equal(t, ConfirmedOrder, order.BrokerState)
		}

		t.Log("Sim broker: tick with time after marker close time")
		{

			order := newTestOpgBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnOpen
			order.Time = time.Date(2010, 1, 1, 8, 10, 10, 0, time.UTC)
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  time.Date(2010, 1, 1, 9, 50, 10, 0, time.UTC),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: false,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderCancelEvent:
				t.Log("OK! Got OrderCancelEvent as expected")
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
			}

			assert.Equal(t, order.Qty, order.BrokerExecQty)
			assert.Equal(t, CanceledOrder, order.BrokerState)
		}

		t.Log("Sim broker: tick with next day time")
		{

			order := newTestOpgBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnOpen
			order.Time = time.Date(2010, 1, 1, 8, 10, 10, 0, time.UTC)
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  time.Date(2010, 1, 4, 8, 50, 10, 0, time.UTC),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: false,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderCancelEvent:
				t.Log("OK! Got OrderCancelEvent as expected")
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
			}

			assert.Equal(t, order.Qty, order.BrokerExecQty)
			assert.Equal(t, CanceledOrder, order.BrokerState)
		}
	}

	t.Log("SHORT MOO order tests")
	{

		t.Log("Sim broker: test normal execution of MOO")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderSell, 200, "ids1")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: true,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, order.Qty, i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
			}

			assert.Equal(t, order.Qty, order.BrokerExecQty)
			assert.Equal(t, FilledOrder, order.BrokerState)

		}

		t.Log("Sim broker: tick that is not MOO")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderSell, 200, "ids2")
			order.Type = MarketOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsOpening: false,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 0)

			assert.Equal(t, int64(0), order.BrokerExecQty)
			assert.Equal(t, ConfirmedOrder, order.BrokerState)
		}
	}
}

func TestSimulatedBroker_fillMocOnTick(t *testing.T) {
	b := newTestSimBrokerWorker()
	t.Log("LONG MOC order tests")
	{

		t.Log("Sim broker: test normal execution of MOC")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderBuy, 200, "id1")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: true,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, order.Qty, i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
			}

			assert.Equal(t, order.Qty, order.BrokerExecQty)
			assert.Equal(t, FilledOrder, order.BrokerState)
		}

		t.Log("Sim broker: tick that is not MOC")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderBuy, 200, "id2")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: false,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 0)
			assert.Equal(t, int64(0), order.BrokerExecQty)
			assert.Equal(t, ConfirmedOrder, order.BrokerState)
		}
	}

	t.Log("SHORT MOC order tests")
	{

		t.Log("Sim broker: test normal execution of MOC")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderSell, 200, "ids1")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: true,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 1)
			assert.Len(t, errors, 0)

			switch i := events[0].(type) {
			case *OrderFillEvent:
				t.Log("OK! Got OrderFillEvent as expected")
				assert.Equal(t, tick.LastPrice, i.Price)
				assert.Equal(t, order.Qty, i.Qty)
				assert.Equal(t, order.Id, i.OrdId)
			default:
				t.Errorf("Error! Expected OrderFillEvent. Got: %+v", i)
			}

			assert.Equal(t, order.Qty, order.BrokerExecQty)
			assert.Equal(t, FilledOrder, order.BrokerState)
		}

		t.Log("Sim broker: tick that is not MOC")
		{
			order := newTestOpgBrokerOrder(math.NaN(), OrderSell, 200, "ids2")
			order.Type = MarketOnClose
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				Symbol:    "Test",
				Datetime:  newTestOpgOrderTime().Add(time.Minute * 5),
				LastPrice: 20.09,
				LastSize:  2000,
				BidPrice:  20.08,
				AskPrice:  20.12,
				IsClosing: false,
			}

			events, errors := putOrderAndFillOnTick(b, order, &tick)
			assert.Len(t, events, 0)
			assert.Len(t, errors, 0)

			assert.Equal(t, int64(0), order.BrokerExecQty)
			assert.Equal(t, ConfirmedOrder, order.BrokerState)
		}
	}
}

/*
func TestSimulatedBroker_checkOnTickLimitAuction(t *testing.T) {
	b := newTestSimBrokerWorker()
	b.marketOpenUntilTime = TimeOfDay{9, 33, 0}
	b.marketCloseUntilTime = TimeOfDay{16, 03, 0}
	t.Log("Sim broker: test auction orders LONG")
	{
		t.Log("Complete execution")
		{
			order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "id1")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.80,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.fillOnTickLimitAuction(order, &tick)
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
			order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "id2")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.90,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),


				IsOpening: true,
			}

			v := b.fillOnTickLimitAuction(order, &tick)
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

				order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "st10")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.fillOnTickLimitAuction(order, &tick)
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

				order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "st2")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.fillOnTickLimitAuction(order, &tick)
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
			order := newTestGtcBrokerOrder(15.87, OrderBuy, 1000, "id1z")
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

			v := b.fillOnTickLimitAuction(order, &tick)
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
			order := newTestGtcBrokerOrder(15.87, OrderSell, 200, "ids1")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.90,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.fillOnTickLimitAuction(order, &tick)
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
			order := newTestGtcBrokerOrder(15.87, OrderSell, 200, "id2s")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.20,
				LastSize:  2000,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.fillOnTickLimitAuction(order, &tick)
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
				order := newTestGtcBrokerOrder(15.87, OrderSell, 200, "st1s")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.fillOnTickLimitAuction(order, &tick)
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
				order := newTestGtcBrokerOrder(15.87, OrderSell, 200, "st2s")
				order.Type = LimitOnOpen
				assert.True(t, order.isValid())

				tick := marketdata.Tick{
					LastPrice: 15.87,
					LastSize:  2000,
					BidPrice:  math.NaN(),
					AskPrice:  math.NaN(),
					IsOpening: true,
				}

				v := b.fillOnTickLimitAuction(order, &tick)
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
			order := newTestGtcBrokerOrder(15.87, OrderSell, 1000, "id1v")
			order.Type = LimitOnOpen
			assert.True(t, order.isValid())

			tick := marketdata.Tick{
				LastPrice: 15.92,
				LastSize:  378,
				BidPrice:  math.NaN(),
				AskPrice:  math.NaN(),
				IsOpening: true,
			}

			v := b.fillOnTickLimitAuction(order, &tick)
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
		order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "id2")
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

		v := b.fillOnTickLOC(order, &tick)
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
		order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "id2")
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

		v := b.fillOnTickLOC(order, &tick)
		assert.Nil(t, v)
		assertNoErrorsGeneratedByBroker(t, b)

	}
}

func TestSimulatedBroker_checkOnTickLOO(t *testing.T) {
	b := newTestSimBrokerWorker()
	b.marketOpenUntilTime = TimeOfDay{9, 35, 0}
	t.Log("Sim broker: check LOO cancelation by time")
	{
		order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "id2")
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

		v := b.fillOnTickLOO(order, &tick)
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
		order := newTestGtcBrokerOrder(15.87, OrderBuy, 200, "id2")
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

		v := b.fillOnTickLOO(order, &tick)
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
		order := newTestGtcBrokerOrder(price, ordSide, qty, id)
		order.Type = ordType
		assert.True(t, order.isValid())
		b.orders[order.Id] = order
		return order
	}

	onTick := func(t *marketdata.Tick) ([]error, []event) {
		wg := &sync.WaitGroup{}
		te := NewTickEvent{
			be(t.Datetime, t.Symbol),
			t,
		}
		go func() {
			wg.Add(1)
			b.onTick(&te)
			wg.Done()
		}()

		var errors []error
		var events []event
	LOOP:
		for {
			select {
			case e := <-b.events:
				events = append(events, e)
			case e := <-b.errChan:
				errors = append(errors, e)

			case <-time.After(5 * time.Millisecond):
				break LOOP
			}
		}

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

	symbol := order1.Ticker
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

		assert.False(t, tick.IsValid())
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

		for _, v := range events {
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

	for _, o := range b.orders {
		assert.Equal(t, NewOrder, o.State)
	}

}

*/
