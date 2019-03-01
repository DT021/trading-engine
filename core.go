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

	portfolio        *portfolioHandler
	terminationChan  chan struct{}
	portfolioChan    chan *PortfolioNewPositionEvent
	errChan          chan error
	marketDataChan   chan event
	strategyDone     chan *StrategyFinishedEvent
	log              log.Logger
	engineMode       EngineMode
	globalWaitGroup  *sync.WaitGroup
	histDataTimeBack time.Duration
}

func NewEngine(sp map[string]ICoreStrategy, broker IBroker, md IMarketData, mode EngineMode, logEvents bool) *Engine {
	if logEvents {
		err := createDirIfNotExists("./StrategyLogs")
		if err != nil {
			panic(err)
		}
	}

	errChan := make(chan error)
	strategyDone := make(chan *StrategyFinishedEvent, len(sp))
	portfolioChan := make(chan *PortfolioNewPositionEvent, 5)

	portfolio := newPortfolio()

	var symbols []string
	for k := range sp {
		symbols = append(symbols, k)
	}

	broker.Init(errChan, symbols)

	for _, k := range symbols {
		brokerChan := make(chan event)
		stategyMarketData := make(chan event, 6)
		signalsChan := make(chan event)
		brokerNotifierChan := make(chan struct{})
		notifyBrokerChan := make(chan *BrokerNotifyEvent)
		cc := CoreStrategyChannels{
			errors:     errChan,
			marketdata: stategyMarketData,
			signals:        signalsChan,
			broker:         brokerChan,
			portfolio:      portfolioChan,
			notifyBroker:   notifyBrokerChan,
			brokerNotifier: brokerNotifierChan,
			strategyDone:   strategyDone,
		}
		sp[k].init(cc)
		sp[k].setPortfolio(portfolio)
		if logEvents {
			sp[k].enableEventLogging()
		}

		bs := BrokerSymbolChannels{
			signals:        signalsChan,
			broker:         brokerChan,
			brokerNotifier: brokerNotifierChan,
			notifyBroker:   notifyBrokerChan,
		}

		broker.SetSymbolChannels(k, bs)
	}

	mdChan := make(chan event)
	md.Init(errChan, mdChan)
	md.SetSymbols(symbols)
	eng := Engine{
		symbols:        symbols,
		broker:         broker,
		md:             md,
		strategiesMap:  sp,
		errChan:        errChan,
		marketDataChan: mdChan,
	}

	eng.engineMode = mode
	eng.portfolioChan = portfolioChan
	eng.portfolio = portfolio
	eng.prepareLogger()
	eng.globalWaitGroup = &sync.WaitGroup{}
	eng.histDataTimeBack = time.Duration(20) * time.Minute
	eng.strategyDone = strategyDone

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
		c.globalWaitGroup.Add(1)
		out := fmt.Sprintf("ERROR ||| %v .", err)
		c.log.Print(out)
		c.globalWaitGroup.Done()
	}()
}

func (c *Engine) logMessage(message string) {
	go func() {
		c.globalWaitGroup.Add(1)
		c.log.Print(message)
		c.globalWaitGroup.Done()
	}()
}

func (c *Engine) eCandleOpen(e *CandleOpenEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.receiveMarketData(e)
}

func (c *Engine) eCandleClose(e *CandleCloseEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.receiveMarketData(e)

}

func (c *Engine) eTick(e *NewTickEvent) {
	if e.Tick.Symbol == "" {
		panic("Tick symbol is empty")
	}
	st := c.getSymbolStrategy(e.Tick.Symbol)
	st.receiveMarketData(e)

}

func (c *Engine) eCandleHistory(e *CandlesHistoryEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.receiveMarketData(e)

}

func (c *Engine) eTickHistory(e *TickHistoryEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.receiveMarketData(e)
}

func (c *Engine) eUpdatePortfolio(e *PortfolioNewPositionEvent) {
	go c.portfolio.onNewTrade(e.trade)
}

func (c *Engine) eEndOfData(e *EndOfDataEvent) {
	for _, st := range c.strategiesMap {
		st.receiveMarketData(e)
	}
}

func (c *Engine) errorIsCritical(err error) bool {
	return false
}

func (c *Engine) runStrategies() {
	for _, s := range c.strategiesMap {
		c.globalWaitGroup.Add(1)
		go s.run()
		c.globalWaitGroup.Done()
	}
}

func (c *Engine) Run() {
	c.logMessage("Engine Run")
	c.md.RequestHistoricalData(c.histDataTimeBack)
	c.logMessage("Request historical market data")
	c.md.Run()
	c.logMessage("Market data listen quotes")
	c.broker.SubscribeEvents()
	c.logMessage("Subscribe to broker events")
	c.runStrategies()
	c.logMessage("Run strategies")
	doneStrategies := 0
	totalStrategies := len(c.strategiesMap)

LOOP:
	for {

		select {
		case e := <-c.portfolioChan:
			c.eUpdatePortfolio(e)
		case e := <-c.errChan:
			c.logError(e)
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
				c.eEndOfData(i)
			default:
				continue LOOP
			}
		case e := <-c.strategyDone:
			doneStrategies ++
			fmt.Printf("Strategy %v is done.Left: %v", e.strategy, totalStrategies-doneStrategies)
			if doneStrategies == totalStrategies {
				break LOOP
			}
		}
	}
	c.shutDown()
}

func (c *Engine) shutDown() {
	c.logMessage("Shutting down engine.")
	c.logMessage("Unsubscribe broker events.")
	//c.broker.UnSubscribeEvents()

	c.logMessage("Waiting for goroutines finish their work.")
	c.globalWaitGroup.Wait()
	c.logMessage("All is done!")
}
