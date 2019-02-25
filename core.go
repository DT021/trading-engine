package engine

import (
	"fmt"
	"log"
	"os"
	"sync"
)

type EngineMode string

const(
	BacktestMode EngineMode = "BacktestMode"
	MarketReplayMode EngineMode = "MarketReplayMode"

)

type Engine struct {
	Symbols             []string
	BrokerConnector     IBroker
	MarketDataConnector IMarketData
	StrategyMap         map[string]IStrategy
	terminationChan     chan struct{}
	errChan             chan error
	mdChan              chan event
	log                 log.Logger
	backtestMode        EngineMode
	workersG            *sync.WaitGroup
}

func NewEngine(sp map[string]IStrategy, broker IBroker, md IMarketData, mode EngineMode) *Engine {

	mdChan := make(chan event)
	errChan := make(chan error)

	var symbols []string
	for k := range sp {
		symbols = append(symbols, k)
	}

	broker.Connect(errChan, symbols)

	for _, k := range symbols {
		brokChan := make(chan event)
		requestChan := make(chan event)
		readyMdChan := make(chan struct{})
		brokNotifyChan := make(chan *BrokerNotifyEvent)
		sp[k].Connect(errChan, brokChan, requestChan, brokNotifyChan, readyMdChan)
		broker.SetSymbolChannels(k, requestChan, brokChan, readyMdChan, brokNotifyChan)

	}

	md.Connect(errChan, mdChan)
	md.SetSymbols(symbols)
	eng := Engine{
		Symbols:             symbols,
		BrokerConnector:     broker,
		MarketDataConnector: md,
		StrategyMap:         sp,
		errChan:             errChan,
		mdChan:              mdChan,
	}

	eng.backtestMode = mode
	eng.prepareLogger()
	eng.workersG = &sync.WaitGroup{}

	return &eng
}

func (c *Engine) getSymbolStrategy(symbol string) IStrategy {
	st, ok := c.StrategyMap[symbol]

	if !ok {
		panic("Strategy for %v not found in map")
	}

	return st
}

func (c *Engine) prepareLogger() {
	f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	c.log.SetOutput(f)
}

func (c *Engine) eCandleOpen(e *CandleOpenEvent) {
	/*st := c.getSymbolStrategy(e.Symbol)

	if c.BrokerConnector.IsSimulated() {
		c.BrokerConnector.OnCandleOpen(e)
	}

	st.onCandleOpenHandler(e)*/
	panic("Not implemented")

}

func (c *Engine) eCandleClose(e *CandleCloseEvent) {
	/*st := c.getSymbolStrategy(e.Symbol)

	if c.BrokerConnector.IsSimulated() {
		c.BrokerConnector.OnCandleClose(e)
	}

	st.onCandleCloseHandler(e)*/
	panic("Not implemented")

}

func (c *Engine) eTick(e *NewTickEvent) {
	if e.Tick.Symbol == "" {
		panic("Tick symbol is empty")
	}
	st := c.getSymbolStrategy(e.Tick.Symbol)
	go func() {
		c.workersG.Add(1)
		st.onTickHandler(e)
		c.workersG.Done()
	}()
}

func (c *Engine) errorIsCritical(err error) bool {
	return false
}

func (c *Engine) logError(err error) {
	out := fmt.Sprintf("ERROR ||| %v .", err)
	c.log.Print(out)

}

func (c *Engine) shutDown() {

	c.BrokerConnector.UnSubscribeEvents()
	for _, s := range c.StrategyMap {
		s.Finish()
	}


}

func (c *Engine) runStrategies() {
	for _, s := range c.StrategyMap {
		go s.Run()
	}
}

func (c *Engine) Run() {
	c.MarketDataConnector.Run()
	c.BrokerConnector.SubscribeEvents()
	c.runStrategies()
LOOP:
	for {

		select {
		case e := <-c.mdChan:
			switch i := e.(type) {
			case *NewTickEvent:
				c.eTick(i)
			case *EndOfDataEvent:
				break LOOP
			}
		case e := <-c.errChan:
			c.logError(e)
		}
	}

	c.workersG.Wait()

	c.shutDown()
}
