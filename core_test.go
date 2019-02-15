package engine

import (
	"alex/marketdata"
	"math/rand"
	"testing"
)

type DummyStrategyWithLogic struct {
}

func (d *DummyStrategyWithLogic) OnTick(b *BasicStrategy, tick *marketdata.Tick) {

	if len(b.currentTrade.AllOrdersIDMap) == 0 && tick.LastPrice > 20 {
		price := tick.LastPrice - rand.Float64()
		err := b.NewLimitOrder(price, OrderSell, 100)
		if err != nil {
			panic(err)
		}
	}
}

func newTestStrategyWithLogic(symbol string) *BasicStrategy {
	st := DummyStrategyWithLogic{}
	bs := BasicStrategy{
		Symbol:   symbol,
		NPeriods: 20,
		strategy: &st}

	bs.init()
	eventsChan := make(chan event)
	errorsChan := make(chan error)
	bs.Connect(errorsChan, eventsChan)
	return &bs

}

func TestEngine_Run(t *testing.T) {
	broker := newTestSimulatedBroker()
	md := newTestBTM()
	md.fraction = 1000
	broker.fraction = 1000
	strategyMap := make(map[string]IStrategy)

	for _, s := range md.Symbols {
		st := newTestStrategyWithLogic(s)
		strategyMap[s] = st

	}

	engine := NewEngine(strategyMap, broker, md, true)

	engine.Run()

	t.Log("Engine done!")
}
