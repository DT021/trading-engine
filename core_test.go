package engine

import (
	"alex/marketdata"
)

type DummyStrategyWithLogic struct {
	idToCancel  string
	alreadySent bool
}

func (d *DummyStrategyWithLogic) OnCandleClose(b *BasicStrategy, candle *marketdata.Candle) {

}

func (d *DummyStrategyWithLogic) OnCandleOpen(b *BasicStrategy, price float64) {

}

func (d *DummyStrategyWithLogic) OnTick(b *BasicStrategy, tick *marketdata.Tick) {
	if len(b.currentTrade.AllOrdersIDMap) == 0 && tick.LastPrice > 20 {
		price := tick.LastPrice - 0.5
		_, err := b.NewLimitOrder(price, OrderSell, 100)
		if err != nil {
			panic(err)
		}
	}

	/*pnl := b.GetTotalPnL()
	if pnl != 0 {
		fmt.Println(pnl)
	}*/
	if len(b.currentTrade.FilledOrders) == 1 && d.idToCancel == "" && !d.alreadySent {

		price := tick.LastPrice * 0.95
		ordId, err := b.NewLimitOrder(price, OrderBuy, 200)
		if err != nil {
			panic(err)
		}
		d.idToCancel = ordId
		d.alreadySent = true

	}

	if d.idToCancel != "" && b.OrderIsConfirmed(d.idToCancel) {
		err := b.CancelOrder(d.idToCancel)
		if err != nil {
			panic(err)
		}
		d.idToCancel = ""

	}
}

func newTestStrategyWithLogic(symbol string) *BasicStrategy {
	st := DummyStrategyWithLogic{}
	bs := BasicStrategy{
		symbol:       symbol,
		nPeriods:     20,
		userStrategy: &st}

	brokerChan := make(chan event)
	stategyMarketData := make(chan event, 6)
	signalsChan := make(chan event)
	brokerNotifierChan := make(chan struct{})
	notifyBrokerChan := make(chan *BrokerNotifyEvent)
	errChan := make(chan error)
	strategyDone := make(chan *StrategyFinishedEvent, 1)
	portfolioChan := make(chan *PortfolioNewPositionEvent, 5)
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

	bs.init(cc)
	return &bs

}



/*func TestEngine_Run(t *testing.T) {
	err := os.Remove("log.txt")
	if err != nil {
		t.Error(err)
	}

	count := 0
	for {
		count ++
		if count > 1 {
			break
		}

		broker := newTestSimBroker()
		md := newTestBTM()

		strategyMap := make(map[string]ICoreStrategy)

		for _, s := range md.Symbols {
			st := newTestStrategyWithLogic(s)
			strategyMap[s] = st

		}

		engine := NewEngine(strategyMap, broker, md, BacktestMode, true)
		engine.SetHistoryTimeBack(15 * time.Second)

		engine.Run()

		t.Logf("Engine #%v finished!", count)
		errorsFound := findErrorsInLog()
		if len(errorsFound) == 0 {
			t.Logf("Engine #%v OK! No errors found", count)
		} else {
			t.Errorf("Found %v errors in Engine #%v", len(errorsFound), count)
			for _, err := range errorsFound {
				t.Error(err)
			}
			break
		}

		engine = nil
		time.Sleep(10 * time.Microsecond)

	}

}*/
