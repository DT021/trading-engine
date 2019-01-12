package engine

import (
	"alex/marketdata"
	"sort"
	"time"
)

type IStrategy interface {
	onCandleOpenHandler(e *CandleOpenEvent) []*event
	onCandleCloseHandler(e *CandleCloseEvent) []*event
	onTickHandler(e *NewTickEvent) []*event
	onTickHistoryHandler(e *TickHistoryEvent) []*event
	onCandleHistoryHandler(e *CandleHistoryEvent) []*event

	onOrderFillHandler(e *OrderFillEvent) []*event
	onOrderCancelHandler(e *OrderCancelEvent) []*event
	onOrderConfirmHandler(e *OrderConfirmationEvent) []*event
	onOrderReplacedHandler(e *OrderReplacedEvent) []*event
	onOrderRejectedHandler(e *OrderRejectedEvent) []*event

	onBrokerPositionHandler(e *BrokerPositionUpdateEvent) []*event

	onTimerTickHandler(e *TimerTickEvent) []*event

	ticks() marketdata.TickArray
	candles() marketdata.CandleArray
}

type BasicStrategy struct {
	Symbol          string
	Name            string
	NPeriods        int
	closedTrades    []*Trade
	currentTrade    *Trade
	Ticks           marketdata.TickArray
	Candles         marketdata.CandleArray
	generatedEvents []*event
}

//Strategy API calls
func (b *BasicStrategy) OnCandleClose() {

}

func (b *BasicStrategy) OnCandleOpen() {

}

func (b *BasicStrategy) OnTick() {

}

func (b *BasicStrategy) OpenOrders() []*Order {
	return b.currentTrade.ConfirmedOrders
}

func (b *BasicStrategy) Position() int {
	if b.currentTrade.Type == FlatTrade || b.currentTrade.Type == ClosedTrade {
		return 0
	}
	pos := b.currentTrade.Qty
	if b.currentTrade.Type == ShortTrade {
		pos = -pos
	}

	return pos

}

func (b *BasicStrategy) NewOrder() {

}

func (b *BasicStrategy) CancelOrder(ordID string) {

}

func (b *BasicStrategy) TickIsValid(t *marketdata.Tick) bool {
	return true
}

func (b *BasicStrategy) CandleIsValid(c *marketdata.Candle) bool {
	return true
}

//Market data events

func (b *BasicStrategy) onCandleCloseHandler(e *CandleCloseEvent) []*event {
	if e == nil {
		return nil
	}
	if !b.CandleIsValid(e.Candle) || e.Candle == nil {
		return nil
	}

	b.putNewCandle(e.Candle)
	b.OnCandleClose()
	return b.flushEvents()

}

func (b *BasicStrategy) onCandleOpenHandler(e *CandleOpenEvent) []*event {
	b.OnCandleOpen()
	return b.flushEvents()

}

//onCandleHistoryHandler puts historical candles in current array of candles.
func (b *BasicStrategy) onCandleHistoryHandler(e *CandleHistoryEvent) []*event {
	if e.Candles == nil {
		return nil
	}
	if len(e.Candles) == 0 {
		return nil
	}

	allCandles := append(b.Candles, e.Candles...)
	listedCandleTimes := make(map[time.Time]struct{})
	var checkedCandles marketdata.CandleArray

	for _, v := range allCandles {
		if v == nil {
			continue
		}
		if !b.CandleIsValid(v) {
			continue
		}
		if _, ok := listedCandleTimes[v.Datetime]; ok {
			continue
		}

		checkedCandles = append(checkedCandles, v)
		listedCandleTimes[v.Datetime] = struct{}{}
	}

	sort.SliceStable(checkedCandles, func(i, j int) bool {
		return checkedCandles[i].Datetime.Unix() < checkedCandles[j].Datetime.Unix()
	})

	if len(checkedCandles) > b.NPeriods {
		b.Candles = checkedCandles[len(checkedCandles)-b.NPeriods:]
	} else {
		b.Candles = checkedCandles
	}

	return nil
}

func (b *BasicStrategy) onTickHandler(e *NewTickEvent) []*event {
	if e == nil {
		return nil
	}
	if !b.TickIsValid(e.Tick) || e.Tick == nil {
		return nil
	}

	b.putNewTick(e.Tick)
	b.OnTick()
	return b.flushEvents()
}

//onTickHistoryHandler puts history ticks in current array of ticks. It doesn't produce any events.
func (b *BasicStrategy) onTickHistoryHandler(e *TickHistoryEvent) []*event {
	if e.Ticks == nil {
		return nil
	}

	if len(e.Ticks) == 0 {
		return nil
	}

	allTicks := append(b.Ticks, e.Ticks...)

	var checkedTicks marketdata.TickArray

	for _, v := range allTicks {
		if v == nil {
			continue
		}
		if !b.TickIsValid(v) {
			continue
		}

		checkedTicks = append(checkedTicks, v)

	}

	sort.SliceStable(checkedTicks, func(i, j int) bool {
		return checkedTicks[i].Datetime.Unix() < checkedTicks[j].Datetime.Unix()
	})

	if len(checkedTicks) > b.NPeriods {
		b.Ticks = checkedTicks[len(checkedTicks)-b.NPeriods:]
	} else {
		b.Ticks = checkedTicks
	}

	return nil
}

//Order events

func (b *BasicStrategy) onOrderFillHandler(e *OrderFillEvent) []*event {
	b.updatePositions(e)
	return b.flushEvents()
}

func (b *BasicStrategy) onOrderCancelHandler(e *OrderCancelEvent) []*event {
	b.updatePositions(e)
	return b.flushEvents()
}

func (b *BasicStrategy) onOrderConfirmHandler(e *OrderConfirmationEvent) []*event {
	b.updatePositions(e)
	return b.flushEvents()
}

func (b *BasicStrategy) onOrderReplacedHandler(e *OrderReplacedEvent) []*event {

	return nil
}

func (b *BasicStrategy) onOrderRejectedHandler(e *OrderRejectedEvent) []*event {

	return nil
}

//Broker events

func (b *BasicStrategy) onBrokerPositionHandler(e *BrokerPositionUpdateEvent) []*event {
	return nil
}

//Timer events

func (b *BasicStrategy) onTimerTickHandler(e *TimerTickEvent) []*event {
	return nil
}

//Private funcs to work with data
func (b *BasicStrategy) flushEvents() []*event { //Todo а не будет ли оно обновлятся?
	if len(b.generatedEvents) > 0 {
		r := b.generatedEvents
		b.generatedEvents = []*event{}
		return r
	}

	return nil
}

func (b *BasicStrategy) updatePositions(e event) {

}

func (b *BasicStrategy) putNewCandle(candle *marketdata.Candle) {
	if candle == nil {
		return
	}

	sortIt := false
	if len(b.Candles) > 0 && candle.Datetime.Before(b.Candles[len(b.Candles)-1].Datetime) {
		sortIt = true
	}

	if len(b.Candles) < b.NPeriods {
		b.Candles = append(b.Candles, candle)
		return
	}
	b.Candles = append(b.Candles[1:], candle)

	if sortIt {
		sort.SliceStable(b.Candles, func(i, j int) bool {
			return b.Candles[i].Datetime.Unix() < b.Candles[j].Datetime.Unix()
		})
	}

	return
}

func (b *BasicStrategy) putNewTick(tick *marketdata.Tick) {
	if tick == nil {
		return
	}
	sortIt := false
	if len(b.Ticks) > 0 && tick.Datetime.Before(b.Ticks[len(b.Ticks)-1].Datetime) {
		sortIt = true
	}

	if len(b.Ticks) < b.NPeriods {
		b.Ticks = append(b.Ticks, tick)
		return
	}
	b.Ticks = append(b.Ticks[1:], tick)

	if sortIt {
		sort.SliceStable(b.Ticks, func(i, j int) bool {
			return b.Ticks[i].Datetime.Unix() < b.Ticks[j].Datetime.Unix()
		})
	}
	return
}

func (b *BasicStrategy) ticks() marketdata.TickArray {
	return b.Ticks
}

func (b *BasicStrategy) candles() marketdata.CandleArray {
	return b.Candles
}
