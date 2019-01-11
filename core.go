package engine

type Engine struct {
	BrokerConnector     IBroker
	MarketDataConnector IMarketData
	Strategy            IStrategy
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

func (c *Engine) IsWaiting() bool {
	if len(c.Events) > 0 {
		return true
	}
	return false
}

func (c *Engine) Run() error {

EVENT_LOOP:
	for {
		if c.MarketDataConnector.Done() && !c.IsWaiting() {
			break EVENT_LOOP
		}
		e := c.nextEvent()
		if e == nil {
			continue EVENT_LOOP
		}

		switch (*e).(type) {
		case *CandleOpenEvent:
			c.Strategy.OnCandleOpen()
		case *CandleCloseEvent:
			c.Strategy.OnCandleClose()
		case *NewOrderEvent:
			c.BrokerConnector.OnNewOrder()
		case *OrderCancelEvent:
			c.Strategy.OnCancel()
		case *NewTickEvent:
			c.Strategy.OnTick()
		case *OrderFillEvent:
			c.Strategy.OnFill()

		}
	}
	return nil
}
