package engine

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"
)

type Engine struct {
	Symbols             []string
	BrokerConnector     IBroker
	MarketDataConnector IMarketData
	StrategyMap         map[string]IStrategy
	brokerChan          chan event
	strategiesChan      chan event
	errChan             chan error
	mdChan              chan event
	lastTime            time.Time
	prevEvent           event
	log                 log.Logger
	backtestMode        bool
	syncChan            chan event
}

func NewEngine(sp map[string]IStrategy, broker IBroker, md IMarketData, mode bool) *Engine {
	brokerChan := make(chan event)
	strategyChan := make(chan event)
	mdChan := make(chan event, 5)
	errChan := make(chan error)

	var symbols []string
	for k := range sp {
		symbols = append(symbols, k)
		sp[k].Connect(errChan, strategyChan)
	}

	broker.Connect(errChan, brokerChan)
	md.Connect(errChan, mdChan)
	md.SetSymbols(symbols)
	eng := Engine{
		Symbols:             symbols,
		BrokerConnector:     broker,
		MarketDataConnector: md,
		StrategyMap:         sp,
		brokerChan:          brokerChan,
		strategiesChan:      strategyChan,
		errChan:             errChan,
		mdChan:              mdChan,
	}

	eng.backtestMode = mode
	eng.prepareLogger()
	eng.syncChan = make(chan event)

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
	st := c.getSymbolStrategy(e.Symbol)

	if c.BrokerConnector.IsSimulated() {
		c.BrokerConnector.OnCandleOpen(e)
	}

	st.onCandleOpenHandler(e)

}

func (c *Engine) eCandleClose(e *CandleCloseEvent) {
	st := c.getSymbolStrategy(e.Symbol)

	if c.BrokerConnector.IsSimulated() {
		c.BrokerConnector.OnCandleClose(e)
	}

	st.onCandleCloseHandler(e)

}

func (c *Engine) eNewOrder(e *NewOrderEvent) {
	c.BrokerConnector.OnNewOrder(e)
}

func (c *Engine) eTick(e *NewTickEvent) {
	if e.Tick.Symbol == "" {
		panic("Tick symbol is empty")
	}
	st := c.getSymbolStrategy(e.Tick.Symbol)

	if c.BrokerConnector.IsSimulated() {
		c.BrokerConnector.OnTick(e.Tick)
	}

	st.onTickHandler(e)

}

func (c *Engine) eFill(e *OrderFillEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.onOrderFillHandler(e)

}

func (c *Engine) eCancelRequest(e *OrderCancelRequestEvent) {
	if c.BrokerConnector.IsSimulated() {
		c.BrokerConnector.OnCancelRequest(e)
	}
}

func (c *Engine) eOrderCanceled(e *OrderCancelEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.onOrderCancelHandler(e)
}

func (c *Engine) eReplaceRequest(e *OrderReplaceRequestEvent) {
	if c.BrokerConnector.IsSimulated() {
		c.BrokerConnector.OnReplaceRequest(e)
	}
}

func (c *Engine) eOrderReplaced(e *OrderReplacedEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.onOrderReplacedHandler(e)
}

func (c *Engine) eOrderConfirmed(e *OrderConfirmationEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.onOrderConfirmHandler(e)
}

func (c *Engine) eOrderRejected(e *OrderRejectedEvent) {
	st := c.getSymbolStrategy(e.Symbol)
	st.onOrderRejectedHandler(e)
}

func (c *Engine) eEndOfData(e *EndOfDataEvent) {
	//TODO
}

func (c *Engine) errorIsCritical(err error) bool {
	return false
}

func (c *Engine) logError(err error) {
	out := fmt.Sprintf("ERROR ||| %v .", err)
	c.log.Print(out)

}

func (c *Engine) shutDown() {
	//Todo

}

func (c *Engine) syncChans() {
	var mdEvent event
	var sigEvent event
	var ordEvent event

EVENT_LOOP:
	for {
		if sigEvent == nil {
			select {
			case e := <-c.strategiesChan:
				sigEvent = e
			case <-time.After(300 * time.Microsecond):
				sigEvent = nil
			//default:
			//	sigEvent = nil
			}

		}
		if ordEvent == nil {
			select {
			case e := <-c.brokerChan:
				ordEvent = e
			case <-time.After(300 * time.Microsecond):
				ordEvent = nil
			//default:
			//	ordEvent = nil
			}
		}
		if mdEvent == nil {
			select {
			case e := <-c.mdChan:
				mdEvent = e
			default:
				mdEvent = nil
			}
		}

		var awaitingEvents []event
	INT_LOOP:
		for _, e := range []event{mdEvent, sigEvent, ordEvent} {
			if e == nil {
				continue INT_LOOP
			}
			awaitingEvents = append(awaitingEvents, e)
		}
		if len(awaitingEvents) == 0 {
			continue EVENT_LOOP
		}

		first := awaitingEvents[0]
		if len(awaitingEvents) > 1 {
			sort.SliceStable(awaitingEvents, func(i, j int) bool {
				return awaitingEvents[i].getTime().Unix() < awaitingEvents[j].getTime().Unix()
			})

			first = awaitingEvents[0]
		}

		c.syncChan <- first

		switch first.(type) {
		case *EndOfDataEvent:
			break EVENT_LOOP

		}

		if first == sigEvent {
			sigEvent = nil
		}

		if first == mdEvent {
			mdEvent = nil
		}

		if first == ordEvent {
			ordEvent = nil
		}
	}
}

func (c *Engine) Run() {
	c.MarketDataConnector.Run()
	go c.syncChans()
EVENT_LOOP:
	for {
		select {
		case e := <-c.syncChan:
			if c.backtestMode {
				msg := fmt.Sprintf("%v ||| %+v", e.getName(), e)
				c.log.Print(msg)
				if e.getTime().Before(c.MarketDataConnector.GetFirstTime()) {
					panic(e)
				}
				t := e.getTime()
				if t.Before(c.lastTime) {
					out := fmt.Sprintf("ERROR|||Events in wrong order: %v, %v, %v, %v", c.prevEvent.getName(),
						c.prevEvent.getTime(), e.getName(), e.getTime())
					c.log.Print(out)
				}
				c.lastTime = t
				c.prevEvent = e
			}

			switch i := e.(type) {

			case *CandleOpenEvent:
				c.eCandleOpen(i)
			case *CandleCloseEvent:
				c.eCandleClose(i)
			case *NewTickEvent:
				c.eTick(i)
			case *EndOfDataEvent:
				c.eEndOfData(i)
				break EVENT_LOOP
			case *NewOrderEvent:
				c.eNewOrder(i)
			case *OrderConfirmationEvent:
				c.eOrderConfirmed(i)
			case *OrderCancelRequestEvent:
				c.eCancelRequest(i)
			case *OrderReplaceRequestEvent:
				c.eReplaceRequest(i)
			case *OrderRejectedEvent:
				c.eOrderRejected(i)
			case *OrderCancelEvent:
				go c.eOrderCanceled(i)
			case *OrderReplacedEvent:
				c.eOrderReplaced(i)
			case *OrderFillEvent:
				c.eFill(i)
			default:
				panic("Unexpected event in events chan")

			}

		case e := <-c.errChan:
			go c.logError(e)
			if c.errorIsCritical(e) {
				break EVENT_LOOP
			}
		}

	}
	c.shutDown()
}
