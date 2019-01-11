package engine

import "alex/marketdata"

type IBroker interface {
	OnNewOrder(e *NewOrderEvent)
	IsSimulated() bool
	OnCandleClose(candle *marketdata.Candle) []*event
	OnCandleOpen(candle *marketdata.Candle) []*event
	OnTick(candle *marketdata.Tick) []*event
}
