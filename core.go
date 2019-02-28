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
	portfolio     *portfolioHandler

	terminationChan chan struct{}
	loggingChan     chan string
	portfolioChan   chan *PortfolioNewPositionEvent
	errChan         chan error
	marketDataChan  chan event
	log             log.Logger
	engineMode      EngineMode
	workersG        *sync.WaitGroup
	globalWaitGroup *sync.WaitGroup
	histDataTimeBack time.Duration
}

func NewEngine(sp map[string]IStrategy, broker IBroker, md IMarketData, mode EngineMode, logEvents bool) *Engine {

	errChan := make(chan error)

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
		signalsChan := make(chan event)
		brokerNotifierChan := make(chan struct{})
		notifyBrokerChan := make(chan *BrokerNotifyEvent)
		cc := CoreStrategyChannels{
			errors:         errChan,
			eventLogging:   eventLoggerChan,
			signals:        signalsChan,
			broker:         brokerChan,
			portfolio:      portfolioChan,
			notifyBroker:   notifyBrokerChan,
			brokerNotifier: brokerNotifierChan,
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
	eng.histDataTimeBack = time.Duration(20) *time.Minute

	return &eng
}

func (c *Engine) getSymbolStrategy(symbol string) IStrategy {
	st, ok := c.strategiesMap[symbol]
	if !ok {
		panic("Strategy for %v not found in map")
	}
	return st
}

func (c *Engine) SetHistoryTimeBack(duration time.Duration){
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
	st := c.getSymbolStrategy(e.Tick.Symbol)
	go func() {
		c.globalWaitGroup.Add(1)
		c.workersG.Add(1)
		st.onTickHandler(e)
		c.workersG.Done()
		c.globalWaitGroup.Done()
	}()
}

func (c *Engine) eCandleHistory(e *CandleHistoryEvent){
	st := c.getSymbolStrategy(e.Symbol)
	go func() {
		c.globalWaitGroup.Add(1)
		c.workersG.Add(1)
		st.onCandleHistoryHandler(e)
		c.workersG.Done()
		c.globalWaitGroup.Done()
	}()

}

func (c *Engine) eTickHistory(e *TickHistoryEvent){
	st := c.getSymbolStrategy(e.Symbol)
	go func() {
		c.globalWaitGroup.Add(1)
		c.workersG.Add(1)
		st.onTickHistoryHandler(e)
		c.workersG.Done()
		c.globalWaitGroup.Done()
	}()
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
	c.md.RequestHistoricalData(c.histDataTimeBack)
	c.md.Run()
	c.broker.SubscribeEvents()
	c.runStrategies()
LOOP:
	for {
		select {
		case e := <-c.portfolioChan:
			c.updatePortfolio(e)
		case e := <-c.loggingChan:
			c.logMessage(e)
		case e := <-c.errChan:
			c.logError(e)

		default:
			select {
			case e := <-c.marketDataChan:
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
					break LOOP
				}
			default:
				continue LOOP

			}
		}
	}
	c.workersG.Wait()
	c.shutDown()
}
