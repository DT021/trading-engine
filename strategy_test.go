package engine

import (
	"fmt"
)



//We use some dummy userStrategy for tests
type DummyStrategy struct {
}

func (d *DummyStrategy) OnTick(b *BasicStrategy, tick *Tick) {

}

func (d *DummyStrategy) OnCandleClose(b *BasicStrategy, candle *Candle) {

}

func (d *DummyStrategy) OnCandleOpen(b *BasicStrategy, price float64) {

}

func newTestBasicStrategy() *BasicStrategy {
	st := DummyStrategy{}
	bs := BasicStrategy{
		symbol:       newTestInstrument(),
		nPeriods:     20,
		userStrategy: &st}


	portfolioChan := make(chan *PortfolioNewPositionEvent, 100)

	cc := CoreStrategyChannels{
		errors:                make(chan error),
		events:                make(chan event),
		portfolio:             portfolioChan,
	}

	bs.init(cc)
	return &bs
}

func isStrategyTicksSorted(st ICoreStrategy) bool {
	tks := st.ticks()
	ok := true
	for i, v := range tks {
		if i == 0 {
			continue
		}

		if tks[i-1].Datetime.After(v.Datetime) {
			fmt.Println("Previous tick is after current: ", tks[i-1].Datetime, v.Datetime)
			ok = false
		}
	}

	return ok
}

/*
func genTickEvents(n int) []event {
	events := make([]event, n, n)
	startTime := time.Now()
	for i := range events {
		tk := marketdata.Tick{
			Datetime:  startTime,
			LastPrice: 200,
			LastSize:  2000,
		}
		startTime = startTime.Add(time.Second * time.Duration(1))
		eTk := NewTickEvent{BaseEvent: be(tk.Datetime, tk.Ticker), Tick: &tk}
		events[i] = &eTk
	}

	return events
}

func genTickArray(n int) marketdata.TickArray {
	//Generate dummy tick array with one nil tick
	ticks := make(marketdata.TickArray, n, n)
	startTime := time.Now()
	for i := range ticks {
		tk := marketdata.Tick{Datetime: startTime, LastPrice:199, LastSize:99}
		startTime = startTime.Add(time.Second * time.Duration(1))

		ticks[i] = &tk
	}

	ticks = append(ticks, nil)

	return ticks
}



func TestBasicStrategy_onTickHandler(t *testing.T) {
	st := newTestBasicStrategy()

	t.Log("Putting first 10 ticks")
	{
		ticksEvents := genTickEvents(10)

		for _, t0 := range ticksEvents {
			wg := &sync.WaitGroup{}
			go func() {
				wg.Add(1)
				st.onTickHandler(t0.(*NewTickEvent))
				wg.Done()
			}()
			t1 := <-st.ch.events //Read is  necessary.
			assert.Equal(t, t0, t1)
			st.ch.brokerNotifier <- struct{}{}
			wg.Wait()
		}

		if len(st.Ticks) != 10 {
			t.Fatalf("\tLen ticks in userStrategy should be 10")
		}
		assert.True(t, isStrategyTicksSorted(st))
	}

	t.Log("Adding 20 more ticks. We expect to have only this 20 new ticks")
	{
		ticksEvents := genTickEvents(20)
		oldestTime := ticksEvents[0].getTime()

		for _, t0 := range ticksEvents {
			wg := &sync.WaitGroup{}
			go func() {
				wg.Add(1)
				st.onTickHandler(t0.(*NewTickEvent))
				wg.Done()
			}()
			t1 := <-st.ch.events //Read is  necessary.
			assert.Equal(t, t0, t1)
			st.ch.brokerNotifier <- struct{}{}
			wg.Wait()
		}

		if len(st.Ticks) != 20 {
			t.Fatalf("\tLen ticks in userStrategy should be 20")
		}

		if st.Ticks[0].Datetime != oldestTime {
			t.Errorf("Expected: %s\n Got %s First stored tick time is wrong", oldestTime, st.Ticks[0].Datetime)
		}
		assert.True(t, isStrategyTicksSorted(st))
	}

	t.Log("Add nil tick event ")
	{
		lastListedTick := st.Ticks[len(st.Ticks)-1]
		st.onTickHandler(nil)

		assert.Equal(t, lastListedTick, st.Ticks[len(st.Ticks)-1], "Ticks are not the same")
		assert.True(t, isStrategyTicksSorted(st))
	}

	t.Log("Add tick event with nil Tick value")
	{
		lastListedTick := st.Ticks[len(st.Ticks)-1]
		st.onTickHandler(&NewTickEvent{Tick: nil})
		assert.Equal(t, lastListedTick, st.Ticks[len(st.Ticks)-1], "Ticks are not the same")
		assert.True(t, isStrategyTicksSorted(st))
	}

}

func TestBasicStrategy_onTickHistoryHandler(t *testing.T) {

	histTicks := genTickArray(30)
	liveTickEvents := genTickEvents(2)
	st := newTestBasicStrategy()

	t.Log("Add first live event")
	{
		t0 := liveTickEvents[0].(*NewTickEvent)
		wg := &sync.WaitGroup{}
		go func() {
			wg.Add(1)
			st.onTickHandler(t0)
			wg.Done()
		}()
		t1 := <-st.ch.events //Read is  necessary.
		assert.Equal(t, t0, t1)
		st.ch.brokerNotifier <- struct{}{}
		wg.Wait()

		assert.Equal(t, 1, len(st.Ticks))
	}

	t.Log("Add history response event")
	{
		e := TickHistoryEvent{BaseEvent: be(histTicks[0].Datetime, "TEST"), Ticks: histTicks}
		st.onTickHistoryHandler(&e)
		assert.Equal(t, 20, len(st.Ticks))
		assert.True(t, isStrategyTicksSorted(st))

	}

	t.Log("Add old live event")
	{
		tm := time.Now().Add(time.Minute * time.Duration(-5))
		oldEvent := &NewTickEvent{BaseEvent: be(tm, "TEST"), Tick: &marketdata.Tick{Datetime: tm, LastPrice:100, LastSize:200}}
		assert.True(t, st.tickIsValid(oldEvent.Tick))
		wg := &sync.WaitGroup{}
		go func() {
			wg.Add(1)
			st.onTickHandler(oldEvent)
			wg.Done()
		}()
		t1 := <-st.ch.events //Read is  necessary.
		assert.Equal(t, oldEvent, t1)
		st.ch.brokerNotifier <- struct{}{}
		wg.Wait()
		assert.Equal(t, 20, len(st.Ticks))
		assert.True(t, isStrategyTicksSorted(st))

	}

	t.Log("Add few new generated events")
	{
		liveTickEvents = genTickEvents(2)

		for _, v := range liveTickEvents {
			t0 := v.(*NewTickEvent)
			wg := &sync.WaitGroup{}
			go func() {
				wg.Add(1)
				st.onTickHandler(t0)
				wg.Done()
			}()
			t1 := <-st.ch.events //Read is  necessary.
			assert.Equal(t, t0, t1)
			st.ch.brokerNotifier <- struct{}{}
			wg.Wait()

		}

		assert.True(t, isStrategyTicksSorted(st))
	}

}

func genCandleArray(n int) marketdata.CandleArray {
	candles := make(marketdata.CandleArray, n, n)
	startTime := time.Now()
	for i := range candles {
		c := marketdata.Candle{Datetime: startTime}
		candles[i] = &c
		startTime = startTime.Add(time.Minute * time.Duration(5))
	}

	candles = append(candles, nil)

	return candles
}

func genCandleCloseEvents(n int) []event {
	events := make([]event, n, n)
	startTime := time.Now()
	for i := range events {
		tk := marketdata.Candle{Datetime: startTime}
		startTime = startTime.Add(time.Second * time.Duration(1))
		eTk := CandleCloseEvent{BaseEvent: be(tk.Datetime, "TEST"), Candle: &tk}
		events[i] = &eTk
	}

	return events

}

func isCandlesSortedAndValid(st ICoreStrategy) (bool, bool) {
	tks := st.candles()
	sortOk := true
	duplicatesOk := true
	listedTimes := make(map[time.Time]struct{})

	for i, v := range tks {
		if _, ok := listedTimes[v.Datetime]; ok {
			duplicatesOk = false
		} else {
			listedTimes[v.Datetime] = struct{}{}
		}
		if i == 0 {
			continue
		}

		if !tks[i-1].Datetime.Before(v.Datetime) {
			fmt.Println("Previous candle is not before current: ", tks[i-1].Datetime, v.Datetime)
			sortOk = false
		}
	}

	return sortOk, duplicatesOk
}

func TestBasicStrategy_onCandleHistoryHandler(t *testing.T) {
	//We have to check we can add both historical and live candles at the same time
	//Internal candle array should be sorted. No duplicate candles (check by candle time)

	st := newTestBasicStrategy()

	basicChecks := func() {
		sorted, valid := isCandlesSortedAndValid(st)
		assert.True(t, sorted)
		assert.True(t, valid)
		assert.Equal(t, st.LastCandleOpen(), st.Candles[len(st.Candles)-1].Open)
	}

	t.Log("Put some historical candles")
	{
		candles := genCandleArray(15)
		e := CandlesHistoryEvent{BaseEvent: be(candles[0].Datetime, "TEST"), Candles: candles}
		st.onCandleHistoryHandler(&e)
		assert.Equal(t, 15, len(st.Candles))
		basicChecks()
	}

	t.Log("Add more historical candles")
	{
		candles := genCandleArray(40)
		e := CandlesHistoryEvent{BaseEvent: be(candles[0].Datetime, "TEST"), Candles: candles}
		st.onCandleHistoryHandler(&e)
		assert.Equal(t, 20, len(st.Candles))
		basicChecks()

		t.Log("Add duplicate candles")
		{
			e2 := CandlesHistoryEvent{BaseEvent: be(candles[0].Datetime, "TEST"), Candles: candles[35:]}
			st.onCandleHistoryHandler(&e2)
			assert.Equal(t, 20, len(st.Candles))
			basicChecks()
		}
	}

	t.Log("Add realtime candles")
	{
		events := genCandleCloseEvents(5)
		for _, e := range events {
			st.onCandleCloseHandler(e.(*CandleCloseEvent))
			assert.Equal(t, 20, len(st.Candles))
			basicChecks()

		}
	}

	t.Log("Send nil realtime event")
	{
		prevLastTime := st.Candles[len(st.Candles)-1].Datetime
		st.onCandleCloseHandler(nil)
		assert.Equal(t, 20, len(st.Candles))
		basicChecks()
		assert.Equal(t, prevLastTime, st.Candles[len(st.Candles)-1].Datetime)
	}

	t.Log("Send realtime event with nil candle")
	{
		prevLastTime := st.Candles[len(st.Candles)-1].Datetime
		st.onCandleCloseHandler(&CandleCloseEvent{Candle: nil})
		assert.Equal(t, 20, len(st.Candles))
		basicChecks()
		assert.Equal(t, prevLastTime, st.Candles[len(st.Candles)-1].Datetime)
	}

}

func TestBasicStrategy_onCandleOpenHandler(t *testing.T) {
	st := newTestBasicStrategy()
	t.Log("Put some historical candles")
	{
		candles := genCandleArray(15)
		e := CandlesHistoryEvent{BaseEvent: be(candles[0].Datetime, "TEST"), Candles: candles}
		st.onCandleHistoryHandler(&e)
		assert.Equal(t, candles[13].Open, st.LastCandleOpen())

	}
	t.Log("Add realtime candle close events")
	{
		candle := &marketdata.Candle{Open: 200.0, Datetime: time.Now().Add(time.Hour * time.Duration(200))}
		st.onCandleCloseHandler(&CandleCloseEvent{Candle: candle})
		assert.Equal(t, 200.0, st.LastCandleOpen())

		candle = &marketdata.Candle{Open: 299.0, Datetime: time.Now().Add(time.Hour * time.Duration(-200))}
		st.onCandleCloseHandler(&CandleCloseEvent{Candle: candle})
		assert.Equal(t, 200.0, st.LastCandleOpen())

	}

	t.Log("Put realtime candle open events")
	{
		e := CandleOpenEvent{Price: 500, CandleTime: time.Now().Add(time.Hour * time.Duration(255))}
		st.onCandleOpenHandler(&e)
		assert.Equal(t, 500.0, st.LastCandleOpen())

		e = CandleOpenEvent{Price: 999, CandleTime: time.Now().Add(time.Hour * time.Duration(200))}
		st.onCandleOpenHandler(&e)
		assert.Equal(t, 500.0, st.LastCandleOpen())

		candle := &marketdata.Candle{Open: 15.0, Datetime: time.Now().Add(time.Hour * time.Duration(600))}
		st.onCandleCloseHandler(&CandleCloseEvent{Candle: candle})
		assert.Equal(t, 15.0, st.LastCandleOpen())

		candle = &marketdata.Candle{Open: 19.0, Datetime: time.Now().Add(time.Hour * time.Duration(100))}
		st.onCandleCloseHandler(&CandleCloseEvent{Candle: candle})
		assert.Equal(t, 15.0, st.LastCandleOpen())
	}

}

func assertNoErrorsGeneratedByEvents(t *testing.T, st *BasicStrategy) {
	select {
	case v, ok := <-st.ch.errors:
		assert.False(t, ok)
		if ok {
			t.Error(v)
		}
	default:
		break
	}
}

func getErrorsGeneratedByStrategy(st *BasicStrategy) error {
	select {
	case v := <-st.ch.errors:
		return v
	case <-time.After(time.Millisecond):
		return nil
	}
}

func TestBasicStrategy_OrdersFlow(t *testing.T) {
	t.Log("Test orders flow in Basic Strategy")
	st := newTestBasicStrategy()

	ord := newTestOrder(100, OrderBuy, 100, "")

	t.Log("Test new order with wrong params")
	{
		wrongOrder := newTestOrder(math.NaN(), OrderBuy, 100, "")
		err := st.newOrder(wrongOrder)

		assertStrategyHasNoEvents(t, st)

		assert.NotNil(t, err)
		assert.Len(t, st.currentTrade.NewOrders, 0)

		wrongOrder.Price = 10
		wrongOrder.Ticker = "Test2"

		err = st.newOrder(wrongOrder)
		assertStrategyHasNoEvents(t, st)

		assert.NotNil(t, err)
		assert.Len(t, st.currentTrade.NewOrders, 0)
	}

	t.Log("Test new order")
	{

		err := st.newOrder(ord)
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)
		assert.Nil(t, err)
		assert.False(t, ord.Id == "")
		assert.Equal(t, 100.0, st.currentTrade.NewOrders[ord.Id].Price)
		assert.Len(t, st.currentTrade.NewOrders, 1)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		assert.Len(t, st.currentTrade.RejectedOrders, 0)
		assert.Len(t, st.currentTrade.CanceledOrders, 0)
		assert.Len(t, st.currentTrade.FilledOrders, 0)
	}

	t.Log("Test confirm event")
	{
		st.onOrderConfirmHandler(&OrderConfirmationEvent{BaseEvent: be(time.Now(), "TEST"), OrdId: ord.Id})
		assert.Equal(t, 100.0, st.currentTrade.ConfirmedOrders[ord.Id].Price)
		assert.Equal(t, ConfirmedOrder, ord.State)
		assert.Len(t, st.currentTrade.NewOrders, 0)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)
		assert.Len(t, st.currentTrade.RejectedOrders, 0)
		assert.Len(t, st.currentTrade.CanceledOrders, 0)
		assert.Len(t, st.currentTrade.FilledOrders, 0)

		assertNoErrorsGeneratedByEvents(t, st)
	}

	t.Log("Test replace event")
	{
		st.onOrderReplacedHandler(&OrderReplacedEvent{
			BaseEvent: be(time.Now(), "TEST"),
			OrdId:     ord.Id,
			NewPrice:  222.0,
		})
		assert.Equal(t, 222.0, st.currentTrade.ConfirmedOrders[ord.Id].Price)
		assert.Equal(t, ConfirmedOrder, ord.State)
		assert.Len(t, st.currentTrade.NewOrders, 0)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)
		assert.Len(t, st.currentTrade.RejectedOrders, 0)
		assert.Len(t, st.currentTrade.CanceledOrders, 0)
		assert.Len(t, st.currentTrade.FilledOrders, 0)
		assert.Equal(t, 222.0, ord.Price)
		assertNoErrorsGeneratedByEvents(t, st)
	}

	t.Log("Test cancel event")
	{
		st.onOrderCancelHandler(&OrderCancelEvent{
			OrdId:     ord.Id,
			BaseEvent: be(time.Now(), "TEST"),
		})

		assert.Equal(t, 222.0, st.currentTrade.CanceledOrders[ord.Id].Price)
		assert.Equal(t, CanceledOrder, ord.State)
		assert.Len(t, st.currentTrade.NewOrders, 0)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		assert.Len(t, st.currentTrade.RejectedOrders, 0)
		assert.Len(t, st.currentTrade.CanceledOrders, 1)
		assert.Len(t, st.currentTrade.FilledOrders, 0)

		assertNoErrorsGeneratedByEvents(t, st)
	}

	t.Log("Test reject event")
	{
		ordToReject := newTestOrder(5, OrderSell, 250, "")
		err := st.newOrder(ordToReject)
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)
		assert.Nil(t, err)

		assert.Equal(t, NewOrder, ordToReject.State)
		assert.Len(t, st.currentTrade.NewOrders, 1)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		assert.Len(t, st.currentTrade.RejectedOrders, 0)
		assert.Len(t, st.currentTrade.CanceledOrders, 1)
		assert.Len(t, st.currentTrade.FilledOrders, 0)

		assert.False(t, ordToReject.Id == "")

		st.onOrderRejectedHandler(&OrderRejectedEvent{OrdId: ordToReject.Id, Reason: "Not shortable", BaseEvent: be(time.Now(), "TEST"),})

		assertNoErrorsGeneratedByEvents(t, st)

		assert.Equal(t, RejectedOrder, ordToReject.State)
		assert.Len(t, st.currentTrade.NewOrders, 0)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		assert.Len(t, st.currentTrade.RejectedOrders, 1)
		assert.Len(t, st.currentTrade.CanceledOrders, 1)
		assert.Len(t, st.currentTrade.FilledOrders, 0)

	}

	t.Log("Test confirm event with wrong ID")
	{
		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: "NotExistingID", BaseEvent: be(time.Now(), "TEST"),})

		err := getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, err)

	}

	t.Log("Test cancel event with wrong ID")
	{
		st.onOrderCancelHandler(&OrderCancelEvent{OrdId: "NotExistingID", BaseEvent: be(time.Now(), "TEST")})

		err := getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, err)
	}

	t.Log("Test replace event with wrong ID")
	{
		st.onOrderReplacedHandler(&OrderReplacedEvent{OrdId: "NotExistingID", NewPrice: 10, BaseEvent: be(time.Now(), "TEST"),})

		err := getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, err)
	}

	t.Log("Test reject event with wrong ID")
	{
		st.onOrderRejectedHandler(&OrderRejectedEvent{OrdId: "NotExistingID", Reason: "SomeReason", BaseEvent: be(time.Now(), "TEST"),})

		err := getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, err)
	}

	t.Log("Test replace and cancel order with wrong status")
	{
		ordTest := newTestOrder(5, OrderSell, 250, "someID")
		err := st.newOrder(ordTest)
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)
		assert.Nil(t, err)

		assert.Equal(t, NewOrder, ordTest.State)
		assert.Len(t, st.currentTrade.NewOrders, 1)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		assert.Len(t, st.currentTrade.RejectedOrders, 1)
		assert.Len(t, st.currentTrade.CanceledOrders, 1)
		assert.Len(t, st.currentTrade.FilledOrders, 0)

		assert.False(t, ordTest.Id == "")

		st.onOrderReplacedHandler(&OrderReplacedEvent{OrdId: ordTest.Id, NewPrice: 10, BaseEvent: be(time.Now(), "TEST"),})

		v := getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, v)

		st.onOrderCancelHandler(&OrderCancelEvent{OrdId: ordTest.Id, BaseEvent: be(time.Now(), "TEST"),})

		v = getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, v)

		assert.Equal(t, NewOrder, ordTest.State)

		st.onOrderRejectedHandler(&OrderRejectedEvent{OrdId: ordTest.Id, Reason: "SomeReason", BaseEvent: be(time.Now(), "TEST"),})

		assert.Equal(t, RejectedOrder, ordTest.State)

		st.onOrderCancelHandler(&OrderCancelEvent{OrdId: ordTest.Id, BaseEvent: be(time.Now(), "TEST"),})

		v = getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, v)

		st.onOrderReplacedHandler(&OrderReplacedEvent{OrdId: ordTest.Id, NewPrice: 10, BaseEvent: be(time.Now(), "TEST"),})

		v = getErrorsGeneratedByStrategy(st)
		assert.NotNil(t, v)

		assert.Equal(t, RejectedOrder, ordTest.State)

	}

}

func assertStrategyHasNewOrderEvent(t *testing.T, st *BasicStrategy) {

	select {
	case v := <-st.ch.events:
		switch v.(type) {
		case *NewOrderEvent:
			t.Log("OK! Has new order event")
		default:
			t.Fatal("FATAL! New order event not produced")
		}
	case <-time.After(time.Second):
		t.Fatal("FATAL! New order event not produced. No events were found in signal chan")


	}
}

func assertStrategyHasNoEvents(t *testing.T, st *BasicStrategy) {
	select {
	case v, ok := <-st.ch.events:
		assert.False(t, ok)
		if ok {
			t.Errorf("ERROR! Expected no events. Found: %v", v)
		}
	case <-time.After(time.Millisecond):
		t.Log("OK! Events chan is empty")
		break
	}
}

func TestBasicStrategy_OrderFillsHandler(t *testing.T) {
	st := newTestBasicStrategy()

	t.Log("Strategy: Add order to flat position and fill it")
	{
		order := newTestOrder(10, OrderBuy, 100, "id1")

		err := st.newOrder(order)
		if err != nil {
			t.Error(err)
		}
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)

		assert.Equal(t, "Test|B|id1", order.Id)
		assert.Equal(t, NewOrder, order.State)
		assert.Equal(t, FlatTrade, st.currentTrade.Type)

		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order.Id, BaseEvent: be(time.Now(), "TEST"),})

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)

		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 11, Qty: 100, BaseEvent: be(time.Now(), order.Ticker)})

		assert.Equal(t, FilledOrder, order.State)
		assert.Equal(t, LongTrade, st.currentTrade.Type)
		assert.Equal(t, int64(100), st.Position())
		assert.Equal(t, 11.0, st.currentTrade.OpenPrice)

		assert.Len(t, st.currentTrade.FilledOrders, 1)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)

		assert.Equal(t, "Test|B|id1", st.currentTrade.Id)

		assert.Len(t, st.closedTrades, 0)
	}

	t.Log("Strategy: Add BUY order to existing LONG and fill it by parts")
	{
		order := newTestOrder(12, OrderBuy, 200, "id2")
		err := st.newOrder(order)
		if err != nil {
			t.Error(err)
		}
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)

		assert.Equal(t, NewOrder, order.State)
		assert.Equal(t, "Test|B|id2", order.Id)

		assert.Len(t, st.currentTrade.NewOrders, 1)

		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order.Id, BaseEvent: be(time.Now(), "TEST"),})

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, st.currentTrade.NewOrders, 0)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)

		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 13, Qty: 50, BaseEvent: be(time.Now(), order.Ticker)})

		assert.Equal(t, int64(150), st.Position())
		assert.Equal(t, LongTrade, st.currentTrade.Type)

		assert.Equal(t, PartialFilledOrder, order.State)
		assert.Equal(t, int64(50), order.ExecQty)

		assert.Equal(t, 13.0, order.ExecPrice)
		assert.Equal(t, 12.0, order.Price)

		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)
		assert.Len(t, st.currentTrade.FilledOrders, 1)

		expectionOpenPrice := (100.0*11.0 + 50.0*13) / 150
		assert.Equal(t, expectionOpenPrice, st.currentTrade.OpenPrice)

		assert.Equal(t, 0.0, st.currentTrade.ClosedPnL)
		assert.Equal(t, expectionOpenPrice*150.0, st.currentTrade.OpenValue)
		assert.Equal(t, 150.0*13.0, st.currentTrade.MarketValue)
		assert.Equal(t, 150.0*13-expectionOpenPrice*150, st.currentTrade.OpenPnL)

		assert.True(t, st.currentTrade.IsOpen())

		//Next fill part
		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 13.5, Qty: 100, BaseEvent: be(time.Now(), order.Ticker)})

		assert.Equal(t, int64(250), st.Position())
		assert.Equal(t, LongTrade, st.currentTrade.Type)

		assert.Equal(t, PartialFilledOrder, order.State)
		assert.Equal(t, int64(150), order.ExecQty)

		expected := (13.0*50 + 13.50*100) / 150
		assert.Equal(t, expected, order.ExecPrice)
		assert.Equal(t, 12.0, order.Price)

		expectionOpenPrice = (100.0*11.0 + 50.0*13 + 100.0*13.5) / 250
		assert.Equal(t, expectionOpenPrice, st.currentTrade.OpenPrice)

		assert.Equal(t, 0.0, st.currentTrade.ClosedPnL)
		assert.Equal(t, expectionOpenPrice*250.0, st.currentTrade.OpenValue)
		assert.Equal(t, 250.0*13.5, st.currentTrade.MarketValue)
		assert.Equal(t, 250.0*13.5-expectionOpenPrice*250, st.currentTrade.OpenPnL)

		assert.True(t, st.currentTrade.IsOpen())

		//Complete fill
		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 11.25, Qty: 50, BaseEvent: be(time.Now(), order.Ticker)})

		assert.Equal(t, int64(300), st.Position())
		assert.Equal(t, LongTrade, st.currentTrade.Type)

		assert.Equal(t, FilledOrder, order.State)
		assert.Equal(t, int64(200), order.ExecQty)

		expected = (13.0*50 + 13.50*100 + 50.0*11.25) / 200
		assert.Equal(t, expected, order.ExecPrice)
		assert.Equal(t, 12.0, order.Price)

		expectionOpenPrice = (100.0*11.0 + 50.0*13 + 100.0*13.5 + 50*11.25) / 300
		assert.Equal(t, expectionOpenPrice, st.currentTrade.OpenPrice)

		assert.Equal(t, 0.0, st.currentTrade.ClosedPnL)
		assert.Equal(t, expectionOpenPrice*300.0, st.currentTrade.OpenValue)
		assert.Equal(t, 300.0*11.25, st.currentTrade.MarketValue)
		assert.Equal(t, 300.0*11.25-expectionOpenPrice*300, st.currentTrade.OpenPnL)

		assert.True(t, st.currentTrade.IsOpen())

		assert.Len(t, st.currentTrade.FilledOrders, 2)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
	}

	t.Log("Strategy: Add executions with wrong params.Check for errors")
	{
		st.onOrderFillHandler(&OrderFillEvent{})
		v := <-st.ch.errors
		t.Logf("OK! Got exception: %v", v)

		st.onOrderFillHandler(&OrderFillEvent{BaseEvent: be(time.Now(), "Test")})
		v = <-st.ch.errors
		t.Logf("OK! Got exception: %v", v)

		st.onOrderFillHandler(&OrderFillEvent{BaseEvent: be(time.Now(), "Test"), OrdId: "Test|B|id1"})
		v = <-st.ch.errors
		t.Logf("OK! Got exception: %v", v)

		st.onOrderFillHandler(&OrderFillEvent{BaseEvent: be(time.Now(), "Test"), OrdId: "Test|B|id1", Price: math.NaN()})
		v = <-st.ch.errors
		t.Logf("OK! Got exception: %v", v)

		st.onOrderFillHandler(&OrderFillEvent{BaseEvent: be(time.Now(), "Test"), OrdId: "Test|B|id1", Price: 10.0})
		v = <-st.ch.errors
		t.Logf("OK! Got exception: %v", v)

	}

	t.Log("Strategy: Partial close of long position")
	{
		order := newTestOrder(10, OrderSell, 100, "ids1")

		prevOpenPrice := st.currentTrade.OpenPrice
		err := st.newOrder(order)
		if err != nil {
			t.Error(err)
		}
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)

		assert.Equal(t, "Test|S|ids1", order.Id)
		assert.Equal(t, NewOrder, order.State)
		assert.Equal(t, LongTrade, st.currentTrade.Type)

		assert.Equal(t, int64(300), st.Position())

		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order.Id, BaseEvent: be(time.Now(), order.Ticker)})

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)

		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Qty: 50, Price: 15.2, BaseEvent: be(time.Now(), order.Ticker)})

		assert.Equal(t, PartialFilledOrder, order.State)
		assert.Equal(t, LongTrade, st.currentTrade.Type)

		assert.Equal(t, int64(250), st.Position())
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)

		assert.Equal(t, 15.2, order.ExecPrice)
		assert.Equal(t, int64(50), order.ExecQty)
		assert.Equal(t, 10.0, order.Price)

		assert.Equal(t, prevOpenPrice, st.currentTrade.OpenPrice)

		assert.Equal(t, prevOpenPrice*250, st.currentTrade.OpenValue)
		assert.Equal(t, 250*15.2, st.currentTrade.MarketValue)

		assert.Equal(t, (15.2-prevOpenPrice)*50, st.currentTrade.ClosedPnL)

		//Add another order and execute it. First sell order still in status partial fill
		//*****************************
		order2 := newTestOrder(15.23, OrderSell, 100, "ids2")
		err = st.newOrder(order2)
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)

		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, NewOrder, order2.State)
		prevClosedPnL := st.currentTrade.ClosedPnL

		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order2.Id})

		assert.Equal(t, ConfirmedOrder, order2.State)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 2)

		st.onOrderFillHandler(&OrderFillEvent{OrdId: order2.Id, Price: order2.Price, Qty: order2.Qty})

		assert.Equal(t, FilledOrder, order2.State)
		assert.Equal(t, int64(100), order2.ExecQty)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)

		assert.Equal(t, int64(150), st.Position())
		assert.Equal(t, prevOpenPrice, st.currentTrade.OpenPrice)
		assert.Equal(t, prevClosedPnL+(order2.Price-prevOpenPrice)*float64(order2.ExecQty), st.currentTrade.ClosedPnL)

		assert.Equal(t, 150*order2.ExecPrice, st.currentTrade.MarketValue)
		//*****************************************************************

		t.Log("Cancel partially filled order")
		{
			st.onOrderCancelHandler(&OrderCancelEvent{OrdId: order.Id})
			assert.Equal(t, CanceledOrder, order.State)
			assert.Equal(t, int64(50), order.ExecQty)
			assert.Len(t, st.currentTrade.CanceledOrders, 1)
			assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		}

		t.Log("Try to add execution for canceled order. Expecting error.")
		{
			st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 10, Qty: 10, BaseEvent: be(time.Now(), order.Ticker)})
			v := <-st.ch.errors
			t.Logf("OK! Got exception: %v", v)
		}
	}

	t.Log("Strategy: Close current LONG position and reverse with single order and partial fills")
	{
		order := newTestOrder(18.16, OrderSell, 500, "ids3")
		err := st.newOrder(order)
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, NewOrder, order.State)
		assert.Equal(t, LongTrade, st.currentTrade.Type)
		assert.Equal(t, int64(150), st.Position())

		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order.Id})

		assert.Equal(t, ConfirmedOrder, order.State)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)
		prevPosId := st.currentTrade.Id
		prevOpenPrice := st.currentTrade.OpenPrice
		prevClosedPnL := st.currentTrade.ClosedPnL

		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 18.20, Qty: 150, BaseEvent: be(time.Now(), order.Ticker)})

		assert.Equal(t, PartialFilledOrder, order.State)
		assert.Equal(t, FlatTrade, st.currentTrade.Type)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 1)
		assert.Len(t, st.currentTrade.FilledOrders, 0)
		assert.Len(t, st.closedTrades, 1)
		assert.Equal(t, prevPosId, st.closedTrades[0].Id)
		assert.NotEqual(t, prevPosId, st.currentTrade.Id)

		prevPos := st.closedTrades[0]

		assert.Equal(t, ClosedTrade, prevPos.Type)
		assert.Equal(t, prevClosedPnL+(order.ExecPrice-prevOpenPrice)*float64(order.ExecQty), prevPos.ClosedPnL)
		assert.Equal(t, 0.0, prevPos.OpenValue)
		assert.Equal(t, 0.0, prevPos.OpenPnL)
		assert.Equal(t, 0.0, prevPos.MarketValue)

		//Complete order fill. Flat position -> short position
		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 18.22, Qty: 350, BaseEvent: be(time.Now(), order.Ticker)})
		assert.Equal(t, ShortTrade, st.currentTrade.Type)
		assert.Equal(t, int64(-350), st.Position())
		assert.Equal(t, 18.22, st.currentTrade.OpenPrice)
		assert.Equal(t, 0.0, st.currentTrade.OpenPnL)
		assert.Equal(t, 0.0, st.currentTrade.ClosedPnL)

		assert.Equal(t, order.Id, st.currentTrade.Id)
		assert.Len(t, st.currentTrade.FilledOrders, 1)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		assert.Equal(t, FilledOrder, order.State)
		assert.Equal(t, (18.22*350+18.20*150)/500, order.ExecPrice)
		assert.Equal(t, 18.16, order.Price)
		assert.Equal(t, int64(500), order.ExecQty)
	}

	t.Log("Strategy: Add to current open SHORT position")
	{
		order := newTestOrder(20.0, OrderSell, 100, "ids5")
		err := st.newOrder(order)
		if err != nil {
			t.Error(err)
		}
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)
		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order.Id})
		assert.Equal(t, ConfirmedOrder, order.State)

		prevValue := st.currentTrade.OpenValue

		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 20.01, Qty: 100})

		assert.Len(t, st.currentTrade.FilledOrders, 2)
		assert.Equal(t, (prevValue+order.ExecPrice*float64(order.ExecQty))/float64(st.currentTrade.Qty), st.currentTrade.OpenPrice)
		assert.Equal(t, 450.0*20.01, st.currentTrade.MarketValue)
		assert.Equal(t, st.currentTrade.OpenValue-st.currentTrade.MarketValue, st.currentTrade.OpenPnL)
		assert.Equal(t, 0.0, st.currentTrade.ClosedPnL)
	}

	t.Log("Strategy: Close current SHORT position. New FLAT position without orders expected")
	{
		order := newTestOrder(19.03, OrderBuy, 450, "idb1")
		err := st.newOrder(order)
		if err != nil {
			t.Error(err)
		}
		assertStrategyHasNewOrderEvent(t, st)
		assertStrategyHasNoEvents(t, st)
		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order.Id})
		assert.Equal(t, ConfirmedOrder, order.State)
		prevOpenPrice := st.currentTrade.OpenPrice

		st.onOrderFillHandler(&OrderFillEvent{OrdId: order.Id, Price: 19.01, Qty: 450})
		assert.Len(t, st.closedTrades, 2)
		assert.Equal(t, FlatTrade, st.currentTrade.Type)
		assert.Equal(t, ClosedTrade, st.closedTrades[1].Type)
		assert.Len(t, st.currentTrade.ConfirmedOrders, 0)
		assert.Len(t, st.currentTrade.NewOrders, 0)
		assert.Len(t, st.currentTrade.FilledOrders, 0)

		assert.Equal(t, (prevOpenPrice-19.01)*450, st.closedTrades[1].ClosedPnL)
		assert.Equal(t, 0.0, st.closedTrades[1].OpenPnL)
		assert.Equal(t, 0.0, st.closedTrades[1].OpenValue)
		assert.Equal(t, 0.0, st.closedTrades[1].MarketValue)
	}
}

func assertStrategyHasCancelRequest(t *testing.T, st *BasicStrategy) {
	v, ok := <-st.ch.events
	if !ok {
		t.Fatal("FATAL! Expected cancel order event.Didn't found any")
	}
	switch v.(type) {
	case *OrderCancelRequestEvent:
		t.Log("OK! Has canel order request event")
	default:
		t.Fatal("FATAL! Canel order request event not produced")
	}
}

func TestBasicStrategy_CancelOrder(t *testing.T) {
	st := newTestBasicStrategy()
	t.Log("Test cancel order request. Normal mode")
	{
		order := newTestOrder(10, OrderSell, 1000, "554")

		err := st.newOrder(order)
		if err != nil {
			t.Error(err)
		}
		assertStrategyHasNewOrderEvent(t, st)
		assertNoErrorsGeneratedByEvents(t, st)

		assertStrategyHasNoEvents(t, st)
		st.onOrderConfirmHandler(&OrderConfirmationEvent{OrdId: order.Id})
		assert.Equal(t, ConfirmedOrder, order.State)
		err = st.CancelOrder(order.Id)
		if err != nil {
			t.Error(err)
		}

		assertStrategyHasCancelRequest(t, st)
		assertStrategyHasNoEvents(t, st)

	}

	t.Log("Test cancel order request with wrong params")
	{
		err := st.CancelOrder("NotExistingID")
		assert.NotNil(t, err)
		assertStrategyHasNoEvents(t, st)

	}

}*/
