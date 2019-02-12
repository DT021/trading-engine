package engine

import (
	"testing"
	"alex/marketdata"
	"fmt"
	"math/rand"
)

type DummyStrategyWithLogic struct {
}

func (d *DummyStrategyWithLogic) OnTick(b *BasicStrategy, tick *marketdata.Tick) {
	fmt.Println("OnTick")

	if len(b.currentTrade.AllOrdersIDMap) == 0 && tick.LastPrice > 20 {
		price := tick.LastPrice - rand.Float64()
		order := Order{
			Side:   OrderSell,
			Qty:    100,
			Symbol: tick.Symbol,
			Price:  price,
			State:  NewOrder,
			Type:   LimitOrder,
			Id:     tick.Datetime.Format("2006-01-02|15:04:05.000") + fmt.Sprintf("%v_%v", tick.Symbol, price),
			Time:   tick.Datetime,
		}
		b.NewOrder(&order)
	}
}

func newTestStrategyWithLogic() *BasicStrategy {
	st := DummyStrategyWithLogic{}
	bs := BasicStrategy{
		Symbol:   "Test",
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
		st := newTestStrategyWithLogic()
		st.Symbol = s
		strategyMap[s] = st
	}

	engine := NewEngine(strategyMap, broker, md, true)

	engine.Run()

	t.Log("Engine done!")
}
