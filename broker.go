package engine

import "alex/marketdata"

type IBroker interface {
	Connect()
	OnNewOrder(e *NewOrderEvent)
	IsSimulated() bool
	OnCandleClose(candle *marketdata.Candle) []*event
	OnCandleOpen(candle *marketdata.Candle) []*event
	OnTick(candle *marketdata.Tick) []*event
	NextEvent()
	PopEvent()
}
