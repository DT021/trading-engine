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

type Engine struct {
	symbols       []string
	broker        IBroker
	md            IMarketData
	strategiesMap map[string]IStrategy
	stratsCals int32
	portfolio *portfolioHandler

	terminationChan  chan struct{}
	loggingChan      chan string
	portfolioChan    chan *PortfolioNewPositionEvent
	errChan          chan error
	marketDataChan   chan event
	strategyDone     chan *StrategyFinishedEvent
	log              log.Logger
	engineMode       EngineMode
	workersG         *sync.WaitGroup
	globalWaitGroup  *sync.WaitGroup
	histDataTimeBack time.Duration
	lastSeenTime     time.Time
}

func NewEngine(sp map[string]IStrategy, broker IBroker, md IMarketData, mode EngineMode, logEvents bool) *Engine {

	errChan := make(chan error)
	strategyDone := make(chan *StrategyFinishedEvent, len(sp))

	portfolioChan := make(chan *PortfolioNewPositionEvent, 5)
	eventLoggerChan := make(chan string)
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
			errors:         errChan,
			marketdata:     stategyMarketData,
			eventLogging:   eventLoggerChan,
			signals:        signalsChan,
			broker:         brokerChan,
			portfolio:      portfolioChan,
			notifyBroker:   notifyBrokerChan,
			brokerNotifier: brokerNotifierChan,
			strategyDone:   strategyDone,
		}
		sp[k].Connect(cc)
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
	eng.workersG = &sync.WaitGroup{}
	eng.globalWaitGroup = &sync.WaitGroup{}
	eng.loggingChan = eventLoggerChan
	eng.histDataTimeBack = time.Duration(20) * time.Minute
	eng.strategyDone = strategyDone

	return &eng
}

func (c *Engine) getSymbolStrategy(symbol string) IStrategy {
	st, ok := c.strategiesMap[symbol]
	if !ok {
		panic("Strategy for %v not found in map")
	}
	return st
}

func (c *Engine) SetHistoryTimeBack(duration time.Duration) {
	c.histDataTimeBack = duration
}

func (c *Engine) prepareLogger() {
	f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	c.log.SetOutput(f)
}

func (c *Engine) eCandleOpen(e *CandleOpenEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	go func() {
		c.globalWaitGroup.Add(1)
		c.workersG.Add(1)
		st.onCandleOpenHandler(e)
		c.workersG.Done()
		c.globalWaitGroup.Done()
	}()
}

func (c *Engine) eCandleClose(e *CandleCloseEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	go func() {
		c.globalWaitGroup.Add(1)
		c.workersG.Add(1)
		st.onCandleCloseHandler(e)
		c.workersG.Done()
		c.globalWaitGroup.Done()
	}()

}

func (c *Engine) eTick(e *NewTickEvent) {
	if e.Tick.Symbol == "" {
		panic("Tick symbol is empty")
	}
	if e.Tick.Datetime.Before(c.lastSeenTime) {
		panic("Wrong tick order in core")
	}
	c.lastSeenTime = e.Tick.Datetime

	st := c.getSymbolStrategy(e.Tick.Symbol)
	st.sendMarketData(e)

}

func (c *Engine) eCandleHistory(e *CandleHistoryEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	go func() {
		c.globalWaitGroup.Add(1)
		c.workersG.Add(1)
		st.onCandleHistoryHandler(e)
		c.workersG.Done()
		c.globalWaitGroup.Done()
	}()

}

func (c *Engine) eTickHistory(e *TickHistoryEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	go func() {
		c.globalWaitGroup.Add(1)
		c.workersG.Add(1)
		st.onTickHistoryHandler(e)
		c.workersG.Done()
		c.globalWaitGroup.Done()
	}()
}

func (c *Engine) eEndOfData(e *EndOfDataEvent) {
	for _, st := range c.strategiesMap {
		st.sendMarketData(e)
	}
}

func (c *Engine) errorIsCritical(err error) bool {
	return false
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

func (c *Engine) shutDown() {
	c.broker.UnSubscribeEvents()
	for _, s := range c.strategiesMap {
		s.Finish()
	}
	c.globalWaitGroup.Wait()
}

func (c *Engine) updatePortfolio(e *PortfolioNewPositionEvent) {
	go c.portfolio.onNewTrade(e.trade)
}

func (c *Engine) runStrategies() {
	for _, s := range c.strategiesMap {
		c.globalWaitGroup.Add(1)
		go s.Run()
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
	//seenEOD := false

LOOP:
	for {
		//fmt.Println("LOOP")
		select {
		case e := <-c.portfolioChan:
			c.updatePortfolio(e)
		case e := <-c.loggingChan:
			c.logMessage(e)
		case e := <-c.errChan:
			c.logError(e)
		case e := <-c.marketDataChan:
			//fmt.Printf(e.getName())
			switch i := e.(type) {
			case *NewTickEvent:
				c.eTick(i)
			case *CandleCloseEvent:
				c.eCandleClose(i)
			case *CandleOpenEvent:
				c.eCandleOpen(i)
			case *CandleHistoryEvent:
				c.eCandleHistory(i)
			case *TickHistoryEvent:
				c.eTickHistory(i)
			case *EndOfDataEvent:
				//seenEOD = true
				c.eEndOfData(i)
			default:
				continue LOOP
			}
		case e :=<-c.strategyDone:
			doneStrategies ++
			fmt.Printf("Strategy %v is done.Left: %v", e.strategy, totalStrategies-doneStrategies)
			if doneStrategies == totalStrategies {
				break LOOP
			}
		}
	}
	fmt.Println("Waiting for goroutines")
	c.workersG.Wait()
	//c.shutDown()
}
