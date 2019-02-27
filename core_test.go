package engine

import (
	"alex/marketdata"
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type DummyStrategyWithLogic struct {
	idToCancel  string
	alreadySent bool
}

func (d *DummyStrategyWithLogic) OnTick(b *BasicStrategy, tick *marketdata.Tick) {
	if len(b.currentTrade.AllOrdersIDMap) == 0 && tick.LastPrice > 20 {
		price := tick.LastPrice - 0.5
		_, err := b.NewLimitOrder(price, OrderSell, 100)
		if err != nil {
			panic(err)
		}
	}

	pnl := b.GetTotalPnL()
	if pnl != 0 {
		fmt.Println(pnl)
	}
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
		Symbol:   symbol,
		NPeriods: 20,
		strategy: &st}

	bs.init()
	/*eventsChan := make(chan event)
	errorsChan := make(chan error)
	bs.Connect(errorsChan, eventsChan, make(chan struct{}))*/
	return &bs

}

func findErrorsInLog() []string {
	file, err := os.Open("log.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()
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

func TestEngine_Run(t *testing.T) {
	err := os.Remove("log.txt")
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for {
		count ++
		if count > 1 {
			break
		}

		broker := newTestSimBroker()
		md := newTestBTM()

		strategyMap := make(map[string]IStrategy)

		for _, s := range md.Symbols {
			st := newTestStrategyWithLogic(s)
			strategyMap[s] = st

		}

		engine := NewEngine(strategyMap, broker, md, BacktestMode, true)

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

}
