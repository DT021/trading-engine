package engine

import (
	"testing"
	"alex/marketdata"
	"time"
	"github.com/stretchr/testify/assert"
	"fmt"
)

//We use some dummy strategy for tests
type DummyStrategy struct {
	BasicStrategy
}

func newTestBasicStrategy() *DummyStrategy {
	bs := BasicStrategy{Symbol: "TEST", NPeriods: 20}
	st := DummyStrategy{bs}
	st.init()
	eventsChan := make(chan *event)
	errorsChan := make(chan error)
	st.connect(eventsChan, errorsChan)
	return &st
}

func genTickEvents(n int) []event {
	events := make([]event, n, n)
	startTime := time.Now()
	for i, _ := range events {
		tk := marketdata.Tick{Datetime: startTime}
		startTime = startTime.Add(time.Second * time.Duration(1))
		eTk := NewTickEvent{Time: tk.Datetime, Tick: &tk}
		events[i] = &eTk
	}

	return events

}

func genTickArray(n int) marketdata.TickArray {
	//Generate dummy tick array with one nil tick
	ticks := make(marketdata.TickArray, n, n)
	startTime := time.Now()
	for i, _ := range ticks {
		tk := marketdata.Tick{Datetime: startTime}
		startTime = startTime.Add(time.Second * time.Duration(1))

		ticks[i] = &tk
	}

	ticks = append(ticks, nil)

	return ticks
}

func isTicksSorted(st IStrategy) bool {
	tks := st.ticks()
	ok := true
	for i, v := range tks {
		//fmt.Println(v.Datetime)
		if i == 0 {
			continue
		}

		if !tks[i-1].Datetime.Before(v.Datetime) {
			fmt.Println("Previous tick is after current: ", tks[i-1].Datetime, v.Datetime)
			ok = false
		}
	}

	return ok
}

func TestBasicStrategy_onTickHandler(t *testing.T) {
	st := newTestBasicStrategy()

	t.Log("Putting first 10 ticks")
	{
		ticksEvents := genTickEvents(10)

		for _, t := range ticksEvents {
			st.onTickHandler(t.(*NewTickEvent))
		}

		if len(st.Ticks) != 10 {
			t.Fatalf("\tLen ticks in strategy should be 10")
		}
		assert.True(t, isTicksSorted(st))
	}

	t.Log("Adding 20 more ticks. We expect to have only this 20 new ticks")
	{
		ticksEvents := genTickEvents(20)
		oldestTime := ticksEvents[0].getTime()

		for _, t := range ticksEvents {
			st.onTickHandler(t.(*NewTickEvent))
		}

		if len(st.Ticks) != 20 {
			t.Fatalf("\tLen ticks in strategy should be 20")
		}

		if st.Ticks[0].Datetime != oldestTime {
			t.Errorf("\t Expected: %s Got %s \tFirst stored tick time is wrong", oldestTime, st.Ticks[0].Datetime)
		}
		assert.True(t, isTicksSorted(st))
	}

	t.Log("Add nil tick event ")
	{
		lastListedTick := st.Ticks[len(st.Ticks)-1]
		st.onTickHandler(nil)
		assert.Equal(t, lastListedTick, st.Ticks[len(st.Ticks)-1], "Ticks are not the same")
		assert.True(t, isTicksSorted(st))
	}

	t.Log("Add tick event with nil Tick value")
	{
		lastListedTick := st.Ticks[len(st.Ticks)-1]
		st.onTickHandler(&NewTickEvent{Tick: nil})
		assert.Equal(t, lastListedTick, st.Ticks[len(st.Ticks)-1], "Ticks are not the same")
		assert.True(t, isTicksSorted(st))
	}

}

func TestBasicStrategy_onTickHistoryHandler(t *testing.T) {

	histTicks := genTickArray(30)
	//time.Sleep(time.Second * time.Duration(2))
	liveTickEvents := genTickEvents(2)

	st := newTestBasicStrategy()

	t.Log("Add first live event")
	{
		st.onTickHandler(liveTickEvents[0].(*NewTickEvent))
		assert.Equal(t, 1, len(st.Ticks))
	}

	t.Log("Add history response event")
	{
		e := TickHistoryEvent{"TEST", histTicks[0].Datetime, histTicks}
		st.onTickHistoryHandler(&e)
		assert.Equal(t, 20, len(st.Ticks))
		assert.True(t, isTicksSorted(st))

	}

	t.Log("Add old live event")
	{
		tm := time.Now().Add(time.Minute * time.Duration(-5))
		oldEvent := NewTickEvent{Time: tm, Tick: &marketdata.Tick{Datetime: tm}}
		st.onTickHandler(&oldEvent)
		assert.Equal(t, 20, len(st.Ticks))
		assert.True(t, isTicksSorted(st))

	}

	t.Log("Add few new generated events")
	{
		liveTickEvents = genTickEvents(2)

		for _, v := range liveTickEvents {
			st.onTickHandler(v.(*NewTickEvent))
		}

		assert.True(t, isTicksSorted(st))
	}

}

func genCandleArray(n int) marketdata.CandleArray {
	candles := make(marketdata.CandleArray, n, n)
	startTime := time.Now()
	for i, _ := range candles {
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
	for i, _ := range events {
		tk := marketdata.Candle{Datetime: startTime}
		startTime = startTime.Add(time.Second * time.Duration(1))
		eTk := CandleCloseEvent{Time: tk.Datetime, Candle: &tk}
		events[i] = &eTk
	}

	return events

}

func isCandlesSortedAndValid(st IStrategy) (bool, bool) {
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
		e := CandleHistoryEvent{Time: candles[0].Datetime, Candles: candles}
		st.onCandleHistoryHandler(&e)
		assert.Equal(t, 15, len(st.Candles))
		basicChecks()
	}

	t.Log("Add more historical candles")
	{
		candles := genCandleArray(40)
		e := CandleHistoryEvent{Time: candles[0].Datetime, Candles: candles}
		st.onCandleHistoryHandler(&e)
		assert.Equal(t, 20, len(st.Candles))
		basicChecks()

		t.Log("Add duplicate candles")
		{
			e2 := CandleHistoryEvent{Time: candles[0].Datetime, Candles: candles[35:]}
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
		e := CandleHistoryEvent{Time: candles[0].Datetime, Candles: candles}
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
