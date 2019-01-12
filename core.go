package engine

type Engine struct {
	BrokerConnector     IBroker
	MarketDataConnector IMarketData
	StrategyMap         map[string]*IStrategy
	Events              []*event
}

func (c *Engine) nextEvent() (*event) {
	if len(c.Events) == 0 {
		return c.MarketDataConnector.Pop()
	}
	lastProduced := *c.Events[0]
	lastMD := *c.MarketDataConnector.Next()
	if lastProduced.getTime().After(lastMD.getTime()) {
		return c.MarketDataConnector.Pop()
	}
	return c.Pop()
}

func (c *Engine) Pop() *event {
	e := c.Events[0]
	if len(c.Events) == 1 {
		c.Events = []*event{}

	} else {
		c.Events = c.Events[1:]
	}
	return e
}

func (c *Engine) Put(e *event) {
	if e == nil {
		return
	}
	c.Events = append(c.Events, e)
}

func (c *Engine) PutMultiply(events []*event) {
	if events == nil {
		return
	}
	if len(events) == 0 {
		return
	}

	c.Events = append(c.Events, events...)
}

func (c *Engine) IsWaiting() bool {
	if len(c.Events) > 0 {
		return true
	}
	return false
}

func (c *Engine) eCandleOpen(e *CandleOpenEvent) {
	st, ok := c.StrategyMap[e.Symbol]

	if !ok {
		// ToDo Logger here
		return
	}

	if c.BrokerConnector.IsSimulated() {
		genEvents := c.BrokerConnector.OnCandleOpen(e.Candle)
		c.PutMultiply(genEvents)
	}

	(*st).onCandleOpenHandler(e)

}

func (c *Engine) eCandleClose(e *CandleCloseEvent) {
	st, ok := c.StrategyMap[e.Symbol]

	if !ok {
		// ToDo Logger here
		return
	}

	if c.BrokerConnector.IsSimulated() {
		genEvents := c.BrokerConnector.OnCandleClose(e.Candle)
		c.PutMultiply(genEvents)
	}

	(*st).onCandleCloseHandler(e)

}

func (c *Engine) eNewOrder(e *NewOrderEvent) {
	c.BrokerConnector.OnNewOrder(e) //todo подумать тут
}

func (c *Engine) eCancelOrder(e *OrderCancelEvent) {

}

func (c *Engine) eTick(e *NewTickEvent) {

}

func (c *Engine) eFill(e *OrderFillEvent) {
	st, ok := c.StrategyMap[e.Symbol]

	if !ok {
		// ToDo Logger here
		return
	}

	(*st).onOrderFillHandler(e)

}

func (c *Engine) Run() error {
    go c.BrokerConnector.Connect()
    go c.MarketDataConnector.Run()
EVENT_LOOP:
	for {
		if c.MarketDataConnector.Done() && !c.IsWaiting() {
			break EVENT_LOOP
		}
		e := c.nextEvent()
		if e == nil {
			continue EVENT_LOOP
		}

		switch i := (*e).(type) {
		case *CandleOpenEvent:
			c.eCandleOpen(i)
		case *CandleCloseEvent:
			c.eCandleClose(i)
		case *NewOrderEvent:
			c.eNewOrder(i)
		case *OrderCancelEvent:
			c.eCancelOrder(i)
		case *NewTickEvent:
			c.eTick(i)
		case *OrderFillEvent:
			c.eFill(i)

		}
	}
	return nil
}
