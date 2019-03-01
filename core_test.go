package engine

import (
	"alex/marketdata"
	"bufio"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
	"time"
)

type DummyStrategyWithLogic struct {
	idToCancel           string
	idToReplace          string
	alreadySentToCancel  bool
	alreadySentToReplace bool
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
	if len(b.currentTrade.FilledOrders) == 1 && d.idToCancel == "" && !d.alreadySentToCancel {
		price := tick.LastPrice * 0.95
		ordId, err := b.NewLimitOrder(price, OrderBuy, 200)
		if err != nil {
			panic(err)
		}
		d.idToCancel = ordId
		d.alreadySentToCancel = true

	}

	if d.idToCancel != "" && b.IsOrderConfirmed(d.idToCancel) {
		err := b.CancelOrder(d.idToCancel)
		if err != nil {
			panic(err)
		}
		d.idToCancel = ""
		return
	}

	if d.idToCancel == "" && d.alreadySentToCancel && !d.alreadySentToReplace {
		price := tick.LastPrice * 0.94
		ordId, err := b.NewLimitOrder(price, OrderBuy, 200)
		if err != nil {
			panic(err)
		}
		d.idToReplace = ordId
		d.alreadySentToReplace = true
	}

	if d.idToReplace != "" && b.IsOrderConfirmed(d.idToReplace) {
		price := tick.LastPrice * 0.99
		err := b.ReplaceOrder(d.idToReplace, price)
		if err != nil {
			panic(err)
		}
		d.idToReplace = ""
		return
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

func findErrorsInLog() []string {
	file, err := os.Open("log.txt")
	if err != nil {
		panic(err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}()
	var errors []string

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		meta := strings.Split(line, "|||")[0]
		if strings.Contains(meta, "ERROR") {
			errors = append(errors, line)
		}
	}
	return errors

}

func assertStrategyWorksCorrect(t *testing.T, genEvents []event) {
	var prevEvent event
	var prevMarketData event
	assert.True(t, len(genEvents) > 0)
	for _, e := range genEvents {
		switch v := e.(type) {
		case *NewTickEvent:
			if prevMarketData == nil {
				prevMarketData = v
				continue
			}
			assert.False(t, v.Tick.Datetime.Before(prevMarketData.getTime()))
			prevMarketData = v
			continue
		case *NewOrderEvent:
			if prevEvent != nil {
				assert.IsType(t, &OrderFillEvent{}, prevEvent)
			}
			prevEvent = v
		case *OrderConfirmationEvent:
			assert.IsType(t, &NewOrderEvent{}, prevEvent)
			assert.True(t, v.getTime().After(prevEvent.getTime()))
			prevEvent = v
		case *OrderFillEvent:
			switch pv := prevEvent.(type) {
			case *OrderConfirmationEvent:
				assert.True(t, v.getTime().After(prevEvent.getTime()))
				prevEvent = v
			case *OrderFillEvent:
				assert.Equal(t, pv.OrdId, v.OrdId)
				prevEvent = v
			case *OrderReplacedEvent:
				assert.Equal(t, pv.OrdId, v.OrdId)
				prevEvent = v
			default:
				t.Errorf("Unexpected event type: %v", v)

			}
		case *OrderCancelRequestEvent:
			assert.IsType(t, &OrderConfirmationEvent{}, prevEvent)
			prevEvent = v
		case *OrderReplaceRequestEvent:
			assert.IsType(t, &OrderConfirmationEvent{}, prevEvent)
			prevEvent = v
		case *OrderCancelEvent:
			assert.IsType(t, &OrderCancelRequestEvent{}, prevEvent)
			assert.True(t, v.getTime().After(prevEvent.getTime()))
			prevEvent = nil
		case *OrderReplacedEvent:
			assert.IsType(t, &OrderReplaceRequestEvent{}, prevEvent)
			assert.True(t, v.getTime().After(prevEvent.getTime()))
			prevEvent = v

		}
	}
}

func TestEngine_RunSimple(t *testing.T) {
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
		eventWritesMap := make(map[string]*eventsSliceStorage)

		for _, s := range md.Symbols {
			st := newTestStrategyWithLogic(s)
			st.enableEventSliceStorage()
			eventWritesMap[s] = &st.eventsSlice
			strategyMap[s] = st

		}

		engine := NewEngine(strategyMap, broker, md, BacktestMode, true)
		engine.SetHistoryTimeBack(15 * time.Second)

		engine.Run()

		t.Logf("Engine #%v finished!", count)
		for k, st := range eventWritesMap {
			t.Logf("Checking %v strategy", k)
			assertStrategyWorksCorrect(t, st.storedEvents())
		}

		errors := findErrorsInLog()
		assert.Len(t, errors, 0)

		for _, st := range strategyMap {
			assert.True(t, isStrategyTicksSorted(st))
		}

	}

}
