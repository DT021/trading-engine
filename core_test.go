package engine

import (
	"alex/marketdata"
	"os"
	"sync"
	"testing"
)

type DummyStrategyWithLogic struct {
}

func (d *DummyStrategyWithLogic) OnTick(b *BasicStrategy, tick *marketdata.Tick) {

	if len(b.currentTrade.AllOrdersIDMap) == 0 && tick.LastPrice > 20 {
		price := tick.LastPrice - 0.5
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
	bs.Connect(errorsChan, eventsChan, &sync.Mutex{})
	return &bs

}
/*Что происходит в брокере - падает эвент, мы генерим че-то и параллельно падает еще эвент,
мапы и ордера не успевают апдейтнуться*/

func TestEngine_Run(t *testing.T) {
	os.Remove("log.txt")
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
