package engine

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"time"
)

type Engine struct {
	Symbols             []string
	BrokerConnector     IBroker
	MarketDataConnector IMarketData
	StrategyMap         map[string]IStrategy
	brokerChans         map[string]chan event
	strategiesChan      map[string]chan event
	errChan             chan error
	mdChan              chan event

	log          log.Logger
	backtestMode bool
	prevEvent    event
}

func NewEngine(sp map[string]IStrategy, broker IBroker, md IMarketData, mode bool) *Engine {

	strategyChan := make(map[string]chan event)
	mdChan := make(chan event)
	errChan := make(chan error)

	var symbols []string
	for k := range sp {
		symbols = append(symbols, k)
		c := make(chan event)
		sp[k].Connect(errChan, c)
		strategyChan[k] = c
	}

	broker.Connect(errChan, symbols)
	md.Connect(errChan, mdChan)
	md.SetSymbols(symbols)
	eng := Engine{
		Symbols:             symbols,
		BrokerConnector:     broker,
		MarketDataConnector: md,
		StrategyMap:         sp,
		brokerChans:         broker.GetAllChannelsMap(),
		strategiesChan:      strategyChan,
		errChan:             errChan,
		mdChan:              mdChan,
	}

	eng.backtestMode = mode
	eng.prepareLogger()

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

}

func (c *Engine) proxyEvent(e event) {
	switch i := e.(type) {

	case *CandleOpenEvent:
		c.eCandleOpen(i)
	case *CandleCloseEvent:
		c.eCandleClose(i)
	case *NewTickEvent:
		c.eTick(i)
	case *EndOfDataEvent:
		c.eEndOfData(i)
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
}

func (c *Engine) checkForErrorsEventQ(e event) {
	if c.backtestMode {
		msg := fmt.Sprintf("%v ||| %+v", e.getName(), e)
		c.log.Print(msg)

		if e.getTime().Before(c.prevEvent.getTime()) {
			out := fmt.Sprintf("ERROR|||Events in wrong order: %v, %v, %v, %v", c.prevEvent.getName(),
				c.prevEvent.getTime(), e.getName(), e.getTime())
			c.log.Print(out)
		}

		c.prevEvent = e
	}

}

func (c *Engine) genEventAfterExpired(w *EngineWaiter) {

}

func (c *Engine) Run() {
	c.MarketDataConnector.Run()
	stratWaiters := make(map[string]*EngineWaiter)
	var bufferedMD []event
	maxL := 20

EVENT_LOOP:
	for {

		//CASE WHEN WE HAVE WAITING RESPONSES FROM BROKER
		if len(stratWaiters) > 0 {
			withMD := false
			l := len(stratWaiters) + 1 //Because we need default chan as well
			if len(bufferedMD) < maxL {
				l += 1 //add MD chan to select case
				withMD = true
			}
			cases := make([]reflect.SelectCase, l)
			casesID := make([]string, l)

			i := 0
			for w := range stratWaiters {
				ch := c.brokerChans[w]
				cases[i] = reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(ch),
				}
				i++
				casesID[i] = w
			}

			//Add Default case
			{
				cases[l-1] = reflect.SelectCase{
					Dir: reflect.SelectDefault,
				}
				casesID[l-1] = "Default"

			}

			if withMD {
				cases[l] = reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(c.mdChan),
				}
				casesID[l] = "MarketData"
			}

			switch index, value, recvOK := reflect.Select(cases); index {
			case l: //Market data case
				ev := reflect.ValueOf(value).Interface().(event)
				switch e := ev.(type) {
				case *NewTickEvent:
					if _, ok := stratWaiters[e.Symbol]; ok {
						bufferedMD = append(bufferedMD, e) //Save it to buffer
					} else {
						c.proxyEvent(e)
					}
				case *EndOfDataEvent:
					bufferedMD = append(bufferedMD, e) //We read it only from buffer
				default:
					panic("Unexpected event for market data")
				}
			case l - 1: // We GOT default value
				if recvOK {
					panic("Shouldn't expect value here : 283")
				}

			default: // Otherwise we get broker event
				ev := reflect.ValueOf(value).Interface().(event)
				switch e := ev.(type) {
				case *OrderFillEvent:
					c.proxyEvent(e) //Just proxy event. Probably we are waiting for another one
				default:
					c.proxyEvent(e)
					delete(stratWaiters, casesID[index])
				}

			}

			if len(stratWaiters) == 0{continue EVENT_LOOP} // We had clear all events

			now := time.Now()
			var kr string

		WaitersLoop:
			for k, v := range stratWaiters {
				if v.isExpired(now) {
					kr = k
					break WaitersLoop
				}
			}

			if kr != "" {
				c.genEventAfterExpired(stratWaiters[kr])
				delete(stratWaiters, kr)
				continue EVENT_LOOP
			}

		}

	}
	c.shutDown()
}

type EngineWaiter struct {
	since       time.Time
	maxWaitTime time.Duration
	e           event
}

func (w *EngineWaiter) isExpired(t time.Time) bool {
	return w.since.Sub(t) > w.maxWaitTime
}
