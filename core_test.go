package engine

import "testing"

func TestEngine_Run(t *testing.T) {
	broker := newTestSimulatedBroker()
	md := newTestBTM()
	md.fraction = 1000
	strategyMap := make(map[string]IStrategy)

	for _, s := range md.Symbols {
		st := newTestBasicStrategy()
		st.Symbol = s
		strategyMap[s] = st
	}

	engine := NewEngine(strategyMap, broker, md)
	engine.Run()

	t.Log("Engine done!")
}
