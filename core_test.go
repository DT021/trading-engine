package engine

import (
	"bufio"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type DummyStrategyWithLogic struct {
	idToCancel           string
	idToReplace          string
	alreadySentToCancel  bool
	alreadySentToReplace bool
	markerId             string
}

func (d *DummyStrategyWithLogic) OnCandleClose(b *BasicStrategy, candle *Candle) {
	if d.markerId == "" {
		id, err := b.NewLimitOrder(candle.Close-0.05, OrderBuy, 200, GTCTIF, "ARCA")
		if err != nil {
			panic(err)
		}
		d.markerId = id
	}

	if b.Position() != 0 && d.idToReplace == "" {
		id, err := b.NewMarketOrder(OrderSell, 300, GTCTIF, "ARCA")
		if err != nil {
			panic(err)
		}
		d.idToReplace = id
	}
}

func (d *DummyStrategyWithLogic) OnCandleOpen(b *BasicStrategy, price float64) {

}

func (d *DummyStrategyWithLogic) OnTick(b *BasicStrategy, tick *Tick) {
	if len(b.currentTrade.AllOrdersIDMap) == 0 && tick.LastPrice > 20 {
		price := tick.LastPrice - 0.5
		_, err := b.NewLimitOrder(price, OrderSell, 100, GTCTIF, "ARCA")
		if err != nil {
			panic(err)
		}
	}
	if len(b.currentTrade.FilledOrders) == 1 && d.idToCancel == "" && !d.alreadySentToCancel {
		price := tick.LastPrice * 0.95
		ordId, err := b.NewLimitOrder(price, OrderBuy, 200, GTCTIF, "ARCA")
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
		ordId, err := b.NewLimitOrder(price, OrderBuy, 200, GTCTIF, "ARCA")
		if err != nil {
			panic(err)
		}
		d.idToReplace = ordId
		d.alreadySentToReplace = true
		return
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

	if b.Position() == 300 && d.markerId == "" {
		id, err := b.NewMarketOrder(OrderSell, 300, GTCTIF, "ARCA")
		if err != nil {
			panic(err)
		}

		d.markerId = id
	}

}

func newTestStrategyWithLogic(symbol *Instrument) *BasicStrategy {
	st := DummyStrategyWithLogic{}
	bs := BasicStrategy{
		symbol:       symbol,
		nPeriods:     20,
		userStrategy: &st}

	errChan := make(chan error)
	portfolioChan := make(chan *PortfolioNewPositionEvent, 5)
	cc := CoreStrategyChannels{
		errors:    errChan,
		events:    make(chan event),
		portfolio: portfolioChan,
	}

	bs.init(cc)
	bs.mdChan = make(chan event, 2)
	bs.handlersWaitGroup = &sync.WaitGroup{}
	go func() {
		bs.mdChan <- &NewTickEvent{}
		//bs.mdChan <- &NewTickEvent{}
	}()

	return &bs

}

func newTestLargeBTMTick(folder string) *BTM {
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		panic(err)
	}

	var testSymbols []*Instrument
	for _, f := range files {
		if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			inst := &Instrument{
				Symbol:f.Name(),
			}
			testSymbols = append(testSymbols, inst)
		}
	}

	fromDate := time.Date(2018, 3, 1, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2018, 5, 1, 0, 0, 0, 0, time.UTC)
	storage := mockStorageJSON{folder: folder}
	b := BTM{
		Symbols:   testSymbols,
		Folder:    "./test_data/BTM",
		mode:      MarketDataModeTicksQuotes,
		FromDate:  fromDate,
		ToDate:    toDate,
		Storage:   &storage,
		waitGroup: &sync.WaitGroup{},
	}

	err = createDirIfNotExists(b.Folder)
	if err != nil {
		panic(err)
	}

	errChan := make(chan error)
	eventChan := make(chan event)

	b.Init(errChan, eventChan)

	return &b
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

func assertStrategyOrderFlowIsCorrect(t *testing.T, genEvents []event) {
	prevEventMap := make(map[string]event)
	var prevTickEvent event
	var prevCandleEvent event

	for _, e := range genEvents {
		switch v := e.(type) {
		case *NewTickEvent:
			if prevTickEvent == nil {
				prevTickEvent = v
				continue
			}
			assert.False(t, v.Tick.Datetime.Before(prevTickEvent.getTime()))
			prevTickEvent = v
			continue

		case *CandleOpenEvent:
			if prevCandleEvent == nil {
				prevCandleEvent = v
				continue
			}
			assert.False(t, v.CandleTime.Before(prevCandleEvent.getTime()))
			prevCandleEvent = v
			continue

		case *CandleCloseEvent:
			assert.NotNil(t, prevCandleEvent)
			assert.False(t, v.getTime().Before(prevCandleEvent.getTime()))
			prevCandleEvent = v
			continue

		case *NewOrderEvent:
			if prevEvent, ok := prevEventMap[v.LinkedOrder.Id]; ok {
				t.Errorf("Already has event in map before NewOrderEvent: %+v", prevEvent)
			}
			prevEventMap[v.LinkedOrder.Id] = e
		case *OrderConfirmationEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				assert.IsType(t, &NewOrderEvent{}, prevEvent)
				assert.True(t, v.getTime().After(prevEvent.getTime()))

			} else {
				assert.True(t, v.getTime().Before(prevEvent.getTime()))
				t.Errorf("Expected NewOrderEvent before Confirmation event. Nothing found. %+v", e)
			}
			prevEventMap[v.OrdId] = e
		case *OrderFillEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				switch prevEvent.(type) {
				case *OrderConfirmationEvent:
					assert.True(t, v.getTime().After(prevEvent.getTime()),
						"PreEvent Time %v, Curr event time %v", prevEvent.getTime(), v.getTime())
				case *OrderFillEvent:
					assert.False(t, v.getTime().Before(prevEvent.getTime()),
						"%v PreEvent Time %v, Curr event time %v", prevEvent.getSymbol(), prevEvent.getTime(), v.getTime())
				case *OrderReplacedEvent:
				default:
					t.Errorf("Expected Confirmation or Fill event. Have: %+v", prevEvent)
				}
			} else {
				t.Errorf("Expected Confirmation or Fill event. Have nothing. Event : %+v", e)
			}

			prevEventMap[v.OrdId] = e

		case *OrderCancelEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				switch prevEvent.(type) {
				case *OrderConfirmationEvent:
					assert.True(t, v.getTime().After(prevEvent.getTime()),
						"PreEvent Time %v, Curr event time %v", prevEvent.getTime(), v.getTime())
				case *OrderFillEvent:
					assert.False(t, v.getTime().Before(prevEvent.getTime()),
						"PreEvent Time %v, Curr event time %v", prevEvent.getTime(), v.getTime())
				default:
					t.Errorf("Exprected confirmation event before cancel. found: %+v", prevEvent)
				}

			} else {
				t.Errorf("Expected OrderConfirmationEvent before Cancel event. Nothing found. %+v", e)
			}

			prevEventMap[v.OrdId] = e

		case *OrderReplacedEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				switch prevEvent.(type) {
				case *OrderConfirmationEvent:
					assert.True(t, v.getTime().After(prevEvent.getTime()),
						"PreEvent Time %v, Curr event time %v", prevEvent.getTime(), v.getTime())
				case *OrderFillEvent:
					assert.True(t, v.getTime().After(prevEvent.getTime()),
						"PreEvent Time %v, Curr event time %v", prevEvent.getTime(), v.getTime())
				default:
					t.Errorf("Exprected confirmation or fill event before replace. found: %+v", prevEvent)
				}

			} else {
				t.Errorf("Expected OrderConfirmationEvent before Replace event. Nothing found. %+v", e)
			}

			prevEventMap[v.OrdId] = e

		}
	}
}

func assertStrategyBrokerEventsFlowIsCorrect(t *testing.T, genEvents []event) {
	prevEventMap := make(map[string]event)

	//assert.True(t, len(genEvents) > 0)
	for _, e := range genEvents {
		switch v := e.(type) {
		case *NewOrderEvent:
			if prevEvent, ok := prevEventMap[v.LinkedOrder.Id]; ok {
				t.Errorf("Already has event in map before NewOrderEvent: %+v", prevEvent)
			}
			prevEventMap[v.LinkedOrder.Id] = e
		case *OrderConfirmationEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				assert.IsType(t, &NewOrderEvent{}, prevEvent)
				assert.True(t, v.getTime().After(prevEvent.getTime()))

			} else {
				assert.True(t, v.getTime().Before(prevEvent.getTime()))
				t.Errorf("Expected NewOrderEvent before Confirmation event. Nothing found. %+v", e)
			}
			prevEventMap[v.OrdId] = e
		case *OrderCancelRequestEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				assert.IsType(t, &OrderConfirmationEvent{}, prevEvent)
				assert.True(t, v.getTime().After(prevEvent.getTime()))
			} else {
				assert.True(t, v.getTime().Before(prevEvent.getTime()))
				t.Errorf("Expected NewOrderEvent before Confirmation event. Nothing found. %+v", e)
			}
			prevEventMap[v.OrdId] = e
		case *OrderCancelEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				assert.IsType(t, &OrderCancelRequestEvent{}, prevEvent)
				assert.True(t, v.getTime().After(prevEvent.getTime()))
			} else {
				assert.True(t, v.getTime().Before(prevEvent.getTime()))
				t.Errorf("Expected NewOrderEvent before Confirmation event. Nothing found. %+v", e)
			}
			prevEventMap[v.OrdId] = e

		case *OrderReplaceRequestEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				assert.IsType(t, &OrderConfirmationEvent{}, prevEvent)
				assert.True(t, v.getTime().After(prevEvent.getTime()))
			} else {
				assert.True(t, v.getTime().Before(prevEvent.getTime()))
				t.Errorf("Expected NewOrderEvent before Confirmation event. Nothing found. %+v", e)
			}
			prevEventMap[v.OrdId] = e

		case *OrderReplacedEvent:
			if prevEvent, ok := prevEventMap[v.OrdId]; ok {
				assert.IsType(t, &OrderReplaceRequestEvent{}, prevEvent)
				assert.True(t, v.getTime().After(prevEvent.getTime()))
			} else {
				assert.True(t, v.getTime().Before(prevEvent.getTime()))
				t.Errorf("Expected NewOrderEvent before Confirmation event. Nothing found. %+v", e)
			}

			prevEventMap[v.OrdId] = e

		}
	}
}

func testEngineRun(t *testing.T, md *BTM, txtLogs bool) {
	err := os.Remove("log.txt")
	if err != nil {
		t.Error(err)
	}

	err = os.RemoveAll("./StrategyLogs/")

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
		broker.delay = 8000

		strategyMap := make(map[string]ICoreStrategy)
		eventWritesMap := make(map[string]*eventsSliceStorage)

		for _, s := range md.Symbols {
			st := newTestStrategyWithLogic(s)
			st.enableEventSliceStorage()
			eventWritesMap[s.Symbol] = &st.eventsLoggingSlice
			strategyMap[s.Symbol] = st

		}

		engine := NewEngine(strategyMap, broker, md, BacktestMode, txtLogs)
		engine.SetHistoryTimeBack(15 * time.Second)

		engine.Run()

		t.Logf("Engine #%v finished!", count)
		for _, st := range eventWritesMap {
			//t.Logf("Checking %v strategy", k)
			assertStrategyOrderFlowIsCorrect(t, st.storedEvents())
			assertStrategyBrokerEventsFlowIsCorrect(t, st.storedEvents())
		}

		errors := findErrorsInLog()
		assert.Len(t, errors, 0)

		for _, st := range strategyMap {
			assert.True(t, isStrategyTicksSorted(st))
		}

	}

}

func TestEngine_RunSimpleTicks(t *testing.T) {
	md := newTestBTMforTicks()
	testEngineRun(t, md, true)

}

func TestEngine_RunLargeDataTicks(t *testing.T) {
	md := newTestLargeBTMTick("D:\\MarketData\\json_storage\\ticks\\quotes_trades")
	testEngineRun(t, md, true)
}

func TestEngine_RunDayCandles(t *testing.T) {
	md := newTestBTMforCandles()
	testEngineRun(t, md, true)
}
