package engine

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type EngineMode string

const (
	BacktestMode     EngineMode = "BacktestMode"
	MarketReplayMode EngineMode = "MarketReplayMode"
)

func createDirIfNotExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err := os.MkdirAll(dirPath, os.ModePerm)
		return err
	}
	return nil
}

type Engine struct {
	symbols       []string
	broker        IBroker
	md            IMarketData
	strategiesMap map[string]ICoreStrategy

	portfolio       *portfolioHandler
	terminationChan chan struct{}
	portfolioChan   chan *PortfolioNewPositionEvent
	errChan         chan error
	marketDataChan  chan event

	events     chan event
	log        log.Logger
	engineMode EngineMode

	histDataTimeBack time.Duration
	mut              *sync.Mutex
	waitG            *sync.WaitGroup
}

func NewEngine(sp map[string]ICoreStrategy, broker IBroker, md IMarketData, mode EngineMode, logEvents bool) *Engine {
	if logEvents {
		err := createDirIfNotExists("./StrategyLogs")
		if err != nil {
			panic(err)
		}
	}

	errChan := make(chan error)
	portfolioChan := make(chan *PortfolioNewPositionEvent, 5)
	portfolio := newPortfolio()

	var symbols []string
	for k := range sp {
		symbols = append(symbols, k)
	}

	events := make(chan event, len(symbols))

	broker.Init(errChan, events, symbols)

	syncGroupMap := make(map[string]*sync.WaitGroup)

	for _, k := range symbols {
		syncGroupMap[k] = &sync.WaitGroup{}

		cc := CoreStrategyChannels{
			errors:    errChan,
			events:    events,
			portfolio: portfolioChan,
		}

		sp[k].init(cc)
		sp[k].setPortfolio(portfolio)
		if logEvents {
			sp[k].enableEventLogging()
		}

	}

	mdChan := make(chan event)
	md.Init(errChan, mdChan)
	md.SetSymbols(symbols)
	eng := Engine{
		symbols:        symbols,
		broker:         broker,
		md:             md,
		events:         events,
		strategiesMap:  sp,
		errChan:        errChan,
		marketDataChan: mdChan,
	}

	eng.engineMode = mode
	eng.portfolioChan = portfolioChan
	eng.portfolio = portfolio
	eng.prepareLogger()

	eng.histDataTimeBack = time.Duration(20) * time.Minute

	eng.terminationChan = make(chan struct{})

	eng.mut = &sync.Mutex{}
	eng.waitG = &sync.WaitGroup{}

	return &eng
}

func (c *Engine) SetHistoryTimeBack(duration time.Duration) {
	c.histDataTimeBack = duration
}

func (c *Engine) getSymbolStrategy(symbol string) ICoreStrategy {
	st, ok := c.strategiesMap[symbol]
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

func (c *Engine) logError(err error) {
	go func() {
		c.waitG.Add(1)
		out := fmt.Sprintf("ERROR ||| %v .", err)
		c.log.Print(out)
		c.waitG.Done()
	}()
}

func (c *Engine) logMessage(message string) {
	go func() {
		c.waitG.Add(1)
		c.log.Print(message)
		c.waitG.Done()
	}()
}

func (c *Engine) eCandleOpen(e *CandleOpenEvent) {
	if c.broker.IsSimulated() {
		c.broker.Notify(e)
	}
	st := c.getSymbolStrategy(e.Symbol)
	st.notify(e)
}

func (c *Engine) eCandleClose(e *CandleCloseEvent) {
	if c.broker.IsSimulated() {
		c.broker.Notify(e)
	}
	st := c.getSymbolStrategy(e.Symbol)
	st.notify(e)
}

func (c *Engine) eTick(e *NewTickEvent) {
	if e.Tick.Symbol == "" {
		panic("Tick symbol is empty")
	}

	st := c.getSymbolStrategy(e.Tick.Symbol)

	if c.broker.IsSimulated() {
		c.broker.Notify(e)
	}
	st.notify(e)

}

func (c *Engine) eCandleHistory(e *CandlesHistoryEvent) {

}

func (c *Engine) eTickHistory(e *TickHistoryEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.notify(e)
}

func (c *Engine) eUpdatePortfolio(e *PortfolioNewPositionEvent) {
	c.waitG.Add(1)
	go func() {
		c.portfolio.onNewTrade(e.trade)
		c.waitG.Done()
	}()
}

func (c *Engine) eEndOfData(e *EndOfDataEvent) {
	c.waitG.Add(1)
	go func() {
		c.terminationChan <- struct{}{}
		c.waitG.Done()
	}()

}

func (c *Engine) errorIsCritical(err error) bool {
	return false
}

func (c *Engine) listendMD() {
	fmt.Println("Listen RealTime")
Loop:
	for {
		select {
		case e := <-c.marketDataChan:
			switch i := e.(type) {
			case *NewTickEvent:
				c.eTick(i)
			case *CandleCloseEvent:
				c.eCandleClose(i)
			case *CandleOpenEvent:
				c.eCandleOpen(i)
			case *CandlesHistoryEvent:
				c.eCandleHistory(i)
			case *TickHistoryEvent:
				c.eTickHistory(i)
			case *EndOfDataEvent:
				c.logMessage("EOD event")
				c.eEndOfData(i)
				break Loop
			}

		}
	}

}

func (c *Engine) listenEvents() {
LOOP:
	for {
		select {
		case e := <-c.events:
			c.proxyEvent(e)
		case e := <-c.portfolioChan:
			c.eUpdatePortfolio(e)
		case e := <-c.errChan:
			c.logError(e)
		case <-c.terminationChan:
			c.logMessage("Events loop terminated")
			break LOOP

		}
	}

}

func (c *Engine) proxyEvent(e event) {
	st := c.getSymbolStrategy(e.getSymbol())
	switch e.(type) {
	case *NewOrderEvent:
		c.broker.Notify(e)
	case *OrderCancelRequestEvent:
		c.broker.Notify(e)
	case *OrderReplaceRequestEvent:
		c.broker.Notify(e)
	case *OrderCancelEvent:
		st.notify(e)
	case *OrderCancelRejectEvent:
		st.notify(e)
	case *OrderConfirmationEvent:
		st.notify(e)
	case *OrderReplacedEvent:
		st.notify(e)
	case *OrderReplaceRejectEvent:
		st.notify(e)
	case *OrderRejectedEvent:
		st.notify(e)
	case *OrderFillEvent:
		st.notify(e)

	}
}

func (c *Engine) Run() {
	c.md.Connect()
	c.broker.Connect()
	c.logMessage("Engine Run")
	c.md.RequestHistoricalData(c.histDataTimeBack)
	c.logMessage("Request historical market data")
	c.md.Run()
	c.logMessage("Market data listen quotes")

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		c.listenEvents()
		wg.Done()
		c.logMessage("Events done")
	}()

	go func() {
		c.listendMD()
		wg.Done()
		c.logMessage("MD done")
	}()

	wg.Wait()

	c.broker.Disconnect()
	c.shutDown()

}

func (c *Engine) shutDown() {
	c.logMessage("Shutting down...")
	for _, st := range c.strategiesMap {
		st.shutDown()
	}
	c.broker.shutDown()
	c.md.ShutDown()
	c.waitG.Wait()
	c.logMessage("Done!")
}
