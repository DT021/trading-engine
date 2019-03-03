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
	syncGroupMap  map[string]*sync.WaitGroup

	portfolio        *portfolioHandler
	terminationChan  chan struct{}
	portfolioChan    chan *PortfolioNewPositionEvent
	errChan          chan error
	marketDataChan   chan event
	strategyDone     chan *StrategyFinishedEvent
	events           chan event
	log              log.Logger
	engineMode       EngineMode
	globalWaitGroup  *sync.WaitGroup
	histDataTimeBack time.Duration
	mut              *sync.Mutex
	waitG            *sync.WaitGroup
	prevSymbol       string
	gorotinesCalls   int32
	doneCallsChan    chan string
	waitingMap       map[string]struct{}
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
	events := make(chan event)

	portfolio := newPortfolio()

	var symbols []string
	for k := range sp {
		symbols = append(symbols, k)
	}

	broker.Init(errChan, events, symbols)

	syncGroupMap := make(map[string]*sync.WaitGroup)

	for _, k := range symbols {
		syncGroupMap[k] = &sync.WaitGroup{}

		cc := CoreStrategyChannels{
			errors:                errChan,
			readyAcceptMarketData: make(chan struct{}),
			events:                events,
			portfolio:             portfolioChan,
			strategyDone:          strategyDone,
		}

		/*go func() {
			cc.readyAcceptMarketData <- struct{}{}
		}()*/
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
	eng.globalWaitGroup = &sync.WaitGroup{}
	eng.histDataTimeBack = time.Duration(20) * time.Minute
	eng.strategyDone = strategyDone
	eng.terminationChan = make(chan struct{})
	eng.syncGroupMap = syncGroupMap
	eng.mut = &sync.Mutex{}
	eng.waitG = &sync.WaitGroup{}
	eng.doneCallsChan = make(chan string, 2)
	eng.waitingMap = make(map[string]struct{})

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
	//st := c.getSymbolStrategy(e.Symbol)
	//st.receiveMarketData(e)
}

func (c *Engine) eCandleClose(e *CandleCloseEvent) {

}

func (c *Engine) eTick(e *NewTickEvent) {
	if e.Tick.Symbol == "" {
		panic("Tick symbol is empty")
	}

	st := c.getSymbolStrategy(e.Tick.Symbol)
	c.broker.OnEvent(e)
	st.OnEvent(e)

}

func (c *Engine) onCallDone(symbol string) {
	delete(c.waitingMap, symbol)
}

func (c *Engine) eCandleHistory(e *CandlesHistoryEvent) {

}

func (c *Engine) eTickHistory(e *TickHistoryEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.OnEvent(e)
}

func (c *Engine) eUpdatePortfolio(e *PortfolioNewPositionEvent) {
	go c.portfolio.onNewTrade(e.trade)
}

func (c *Engine) eEndOfData(e *EndOfDataEvent) {
	c.terminationChan <- struct{}{}

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
			//fmt.Println(e.getName() + " " + e.getSymbol())
			msg := fmt.Sprintf("NEW MD || %v || %v", e.getSymbol(), e.getName())
			c.logMessage(msg)
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
				fmt.Println("EOD")
				//c.shutDown()
				go c.eEndOfData(i)
				break Loop
			}
		case e := <-c.doneCallsChan:
			c.onCallDone(e)
		}
	}

}

func (c *Engine) listenEvents() {
LOOP:
	for {
		//fmt.Println("EventLoop")

		select {
		case e := <-c.events:
			go c.proxyEvent(e)
		case e := <-c.portfolioChan:
			c.eUpdatePortfolio(e)
		case e := <-c.errChan:
			c.logError(e)
		case <-c.terminationChan:
			fmt.Println("Terminated")
			break LOOP

		}
	}

}

func (c *Engine) proxyEvent(e event) {
	st := c.getSymbolStrategy(e.getSymbol())
	switch e.(type) {
	case *NewOrderEvent:
		c.broker.OnEvent(e)
	case *OrderCancelRequestEvent:
		c.broker.OnEvent(e)
	case *OrderReplaceRequestEvent:
		c.broker.OnEvent(e)
	case *OrderCancelEvent:
		st.OnEvent(e)
	case *OrderCancelRejectEvent:
		st.OnEvent(e)
	case *OrderConfirmationEvent:
		st.OnEvent(e)
	case *OrderReplacedEvent:
		st.OnEvent(e)
	case *OrderReplaceRejectEvent:
		st.OnEvent(e)
	case *OrderRejectedEvent:
		st.OnEvent(e)
	case *OrderFillEvent:
		st.OnEvent(e)

	}
}

func (c *Engine) Run() {
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
		fmt.Println("Events done")
	}()

	go func() {
		//wg.Add(1)
		c.listendMD()
		wg.Done()
		fmt.Println("MD done")
	}()

	wg.Wait()

	//c.shutDown()
}

func (c *Engine) shutDown() {
	for _, st := range c.strategiesMap {
		st.readyForMD()
	}
}
