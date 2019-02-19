package engine

import (
	"alex/marketdata"
	"sync"
	"testing"
)

type DummyStrategyWithLogic struct {
	idToCancel  string
	alreadySent bool
}

func (d *DummyStrategyWithLogic) OnTick(b *BasicStrategy, tick *marketdata.Tick) {
	if len(b.currentTrade.AllOrdersIDMap) == 0 && tick.LastPrice > 20 {
		price := tick.LastPrice - 0.5
		_, err := b.NewLimitOrder(price, OrderSell, 100)
		if err != nil {
			panic(err)
		}
	}

	if len(b.currentTrade.FilledOrders) == 1 && d.idToCancel == "" && !d.alreadySent {
		price := tick.LastPrice * 0.95
		ordId, err := b.NewLimitOrder(price, OrderBuy, 200)
		if err != nil {
			panic(err)
		}
		d.idToCancel = ordId
		d.alreadySent = true

	}

	if d.idToCancel != "" && b.OrderIsConfirmed(d.idToCancel) {
		err := b.CancelOrder(d.idToCancel)
		if err != nil {
			panic(err)
		}
		d.idToCancel = ""

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

func TestEngine_Run(t *testing.T) {
	/*err := os.Remove("log.txt")
	if err != nil {
		t.Fatal(err)
	}
	broker := newTestSimulatedBroker()
	md := newTestBTM()

	broker.fraction = 1000
	strategyMap := make(map[string]IStrategy)

	for _, s := range md.Symbols {
		st := newTestStrategyWithLogic(s)
		strategyMap[s] = st

	}

	engine := NewEngine(strategyMap, broker, md, true)

	engine.Run()

	t.Log("Engine done!")*/
}
