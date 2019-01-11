package engine

import "alex/marketdata"

type IStrategy interface {
	onCandleOpenHandler(e *CandleOpenEvent)
	onCandleCloseHandler(e *CandleCloseEvent)
	onOrderFill(fillEvent *OrderFillEvent)
	onOrderCancel(cancelEvent *OrderCancelEvent)
	onTickHandler(tickEvent *NewTickEvent)
	onOrderConfirm(confEvent *OrderConfirmationEvent)
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

func (b *BasicStrategy) putNewCandle(candle *marketdata.Candle) {
	if len(b.Candles) < b.NPeriods {
		b.Candles = append(b.Candles, candle)
		return
	}
	b.Candles = append(b.Candles[1:], candle)
	return
}

func (b *BasicStrategy) putNewTick(tick *marketdata.Tick) {
	if len(b.Ticks) < b.NPeriods {
		b.Ticks = append(b.Ticks, tick)
		return
	}
	b.Ticks = append(b.Ticks[1:], tick)
	return
}

func (b *BasicStrategy) onCandleCloseHandler(closeEvent *CandleCloseEvent) []*event {
	b.putNewCandle(closeEvent.Candle)
	b.OnCandleClose()
	return b.flushEvents()

}

func (b *BasicStrategy) onCandleOpenHandler(openEvent *CandleOpenEvent) []*event {
	b.OnCandleOpen()
	return b.flushEvents()

}

func (b *BasicStrategy) onTickHandler(tickEvent *NewTickEvent) []*event {
	b.putNewTick(tickEvent.Tick)
	b.OnTick()
	return b.flushEvents()
}

func (b *BasicStrategy) onOrderFill(fillEvent *OrderFillEvent) []*event {
	b.updatePositions(fillEvent)
	return b.flushEvents()
}

func (b *BasicStrategy) onOrderCancel(cancelEvent *OrderCancelEvent) []*event {
	b.updatePositions(cancelEvent)
	return b.flushEvents()
}

func (b *BasicStrategy) onOrderConfirm(confEvent *OrderConfirmationEvent) []*event {
	b.updatePositions(confEvent)
	return b.flushEvents()
}

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

/*Functions should be overwrite by your strategy*/
func (b *BasicStrategy) OnCandleClose() {

}

func (b *BasicStrategy) OnCandleOpen() {

}

func (b *BasicStrategy) OnTick() {

}
