package engine

import (
	"alex/marketdata"
	"bufio"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMockStorageJSON_GetStoredCandles(t *testing.T) {
	storage := mockStorageJSON{folder: "D:\\MarketData\\json_storage\\candles\\day"}
	candles, err := storage.GetStoredCandles("SPB", "D", marketdata.DateRange{})
	assert.Nil(t, err)
	assert.True(t, len(candles) > 0)
}

func TestBTM_getFilename(t *testing.T) {
	m := BTM{}

	symbols1 := []*Instrument{
		{Symbol: "S1"},
		{Symbol: "S2"},
		{Symbol: "S#"},
	}

	symbols2 := []*Instrument{
		{Symbol: "S2"},
		{Symbol: "S1"},
		{Symbol: "S#"},
	}

	symbols3 := []*Instrument{
		{Symbol: "S1"},
		{Symbol: "S4"},
		{Symbol: "S#"},
	}

	m.Symbols = symbols1
	f1, err := m.getFilename()
	if err != nil {
		t.Error(err)
	}
	m.Symbols = symbols2
	f2, err := m.getFilename()
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, f1, f2)

	m.Symbols = symbols3

	f3, err := m.getFilename()
	if err != nil {
		t.Error(err)
	}

	assert.NotEqual(t, f1, f3)

	fmt.Println(f3)

}

func newTestBTMforTicks() *BTM {
	testSymbolsRaw := []string{
		"Sym1",
		"Sym2",
		"Sym3",
		"Sym4",
		"Sym5",
		"Sym6",
		"Sym7",
	}

	testSymbols := make([]*Instrument, len(testSymbolsRaw))
	for i, v := range testSymbolsRaw {
		testSymbols[i] = &Instrument{
			Symbol: v,
		}
	}

	fromDate := time.Date(2018, 3, 2, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2018, 3, 10, 0, 0, 0, 0, time.UTC)
	storage := mockStorageJSON{folder: "./test_data/json_storage/ticks/quotes_trades"}
	b := BTM{
		Symbols:   testSymbols,
		Folder:    "./test_data/BTM",
		mode:      MarketDataModeTicksQuotes,
		FromDate:  fromDate,
		ToDate:    toDate,
		Storage:   &storage,
		waitGroup: &sync.WaitGroup{},
	}

	err := createDirIfNotExists(b.Folder)
	if err != nil {
		panic(err)
	}

	errChan := make(chan error)
	eventChan := make(chan event)

	b.Init(errChan, eventChan)

	return &b
}

func newTestBTMforCandles() *BTM {
	folder := "./test_data/json_storage/candles"
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		panic(err)
	}

	var testSymbols []*Instrument
	for _, f := range files {
		inst := &Instrument{
			Symbol: strings.Split(f.Name(), ".")[0],
		}
		testSymbols = append(testSymbols, inst)
	}
	fromDate := time.Date(2018, 3, 2, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2018, 3, 10, 0, 0, 0, 0, time.UTC)
	storage := mockStorageJSON{folder: folder}
	b := BTM{
		Symbols:          testSymbols,
		Folder:           "./test_data/BTM",
		mode:             MarketDataModeCandles,
		FromDate:         fromDate,
		ToDate:           toDate,
		Storage:          &storage,
		waitGroup:        &sync.WaitGroup{},
		candlesTimeFrame: "D",
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

func assertNoErrorsGeneratedByBTM(t *testing.T, b *BTM) {
	select {
	case v, ok := <-b.errChan:
		assert.False(t, ok)
		if ok {
			t.Errorf("ERROR! Expected no errors. Found: %v", v)
		}
	default:
		t.Log("OK! Error chan is empty")
		break
	}
}

func prepairedDataIsSorted(pth string, t *testing.T) bool {
	file, err := os.Open(pth)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			t.Error(err)
		}

	}()

	scanner := bufio.NewScanner(file)
	prevTimeUnix := 0
	for scanner.Scan() {
		curTimeUnix, err := strconv.Atoi(strings.Split(scanner.Text(), ",")[0])
		if err != nil {
			t.Error(err)
			continue
		}
		if curTimeUnix < prevTimeUnix {
			t.Logf("Curtime %v is less than prevTime %v", curTimeUnix, prevTimeUnix)
			return false
		}
		prevTimeUnix = curTimeUnix
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	return true
}

func TestBTM_prepare(t *testing.T) {
	startTime := time.Now()

	b := newTestBTMforTicks()
	err := os.Remove(b.getPrepairedFilePath())
	if err != nil {
		t.Error(err)
	}
	_, err = b.getFilename()
	if err != nil {
		t.Error(err)
	}

	b.prepare()
	fi, err := os.Stat(b.getPrepairedFilePath())
	if err != nil {
		t.Error(err)
	}

	assert.True(t, fi.ModTime().UnixNano() > startTime.UnixNano())
	assertNoErrorsGeneratedByBTM(t, b)
	sorted := prepairedDataIsSorted(b.getPrepairedFilePath(), t)
	assert.True(t, sorted)

}

func TestBTM_RunTicks(t *testing.T) {
	b := newTestBTMforTicks()

	err := os.Remove(b.getPrepairedFilePath())
	if err != nil {
		t.Error(err)
	}

	t.Log("Test without history events")
	{
		b.Run()
		time.Sleep(10 * time.Millisecond)
		totalE := 0
		var prevTime time.Time
	LOOP:
		for {
			select {
			case e := <-b.mdChan:
				switch i := e.(type) {
				case *NewTickEvent:
					totalE++
					prevTime = i.getTime()
					assert.False(t, i.getTime().Before(prevTime))
				case *EndOfDataEvent:
					break LOOP
				default:
					t.Errorf("Unexpected event type: %+v", i)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Not found enough ticks")

			}
		}
		assert.Equal(t, 1147, totalE)
	}

	t.Log("Test with history requests")
	{
		b.RequestHistoricalData(2 * time.Second)
		b.Run()
		histResponses := make(map[string]TickArray)
		tickEventsN := 0
		histEventsN := 0
		totalTicks := 0
	HIST_LOOP:
		for {
			select {
			case e := <-b.mdChan:
				switch i := e.(type) {
				case *NewTickEvent:
					tickEventsN ++
					totalTicks ++
					if hist, ok := histResponses[i.Ticker.Symbol]; ok {
						assert.False(t, i.Tick.Datetime.Before(hist[len(hist)-1].Datetime))
					} else {
						t.Errorf("Got NewTickEvent without history response for %v", i.Ticker)
					}
				case *TickHistoryEvent:
					histEventsN ++
					_, ok := histResponses[i.Ticker.Symbol]
					assert.False(t, ok)
					histResponses[i.Ticker.Symbol] = i.Ticks
					totalTicks += len(i.Ticks)
					for n, tick := range i.Ticks {
						assert.Equal(t, tick.Symbol, i.Ticker.Symbol)
						if n == 0 {
							continue
						} else {
							assert.False(t, tick.Datetime.Before(i.Ticks[n-1].Datetime))
						}
					}

				case *EndOfDataEvent:
					break HIST_LOOP
				}
			}
		}

		assert.True(t, tickEventsN > 0)
		assert.True(t, histEventsN > 0)
		assert.Equal(t, 1147, totalTicks)
	}

}

func TestBTM_RunCandles(t *testing.T) {
	b := newTestBTMforCandles()

	err := os.Remove(b.getPrepairedFilePath())
	if err != nil {
		t.Error(err)
	}

	t.Log("Test without history events")
	{
		b.Run()
		//time.Sleep(10 * time.Millisecond)
		//totalE := 0
		//var prevTime time.Time
		symbolEventMap := make(map[string]event)

	LOOP:
		for {
			select {
			case e := <-b.mdChan:
				switch i := e.(type) {
				case *CandleOpenEvent:
					if _, ok := symbolEventMap[i.Ticker.Symbol]; ok {
						assert.IsType(t, &CandleCloseEvent{}, symbolEventMap[i.Ticker.Symbol])
						assert.False(t, i.getTime().Before(symbolEventMap[i.Ticker.Symbol].getTime()))
					}
					symbolEventMap[i.Ticker.Symbol] = e
				case *CandleCloseEvent:
					pe, ok := symbolEventMap[i.Ticker.Symbol]
					assert.True(t, ok)
					assert.IsType(t, &CandleOpenEvent{}, pe)
					assert.True(t, i.getTime().After(pe.getTime()),
						"Prev event time %v, event: %+v, Curr event time %v, event: %+v",
						pe.getTime(), pe, i.getTime(), i)
					symbolEventMap[i.Ticker.Symbol] = e
				case *EndOfDataEvent:
					break LOOP
				default:
					t.Errorf("Unexpected event type: %+v", i)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Not found enough ticks")

			}
		}
		//assert.Equal(t, 1147, totalE)
	}

	/*t.Log("Test with history requests")
	{
		b.RequestHistoricalData(2 * time.Second)
		b.Run()
		histResponses := make(map[string]marketdata.TickArray)
		tickEventsN := 0
		histEventsN := 0
		totalTicks := 0
	HIST_LOOP:
		for {
			select {
			case e := <-b.mdChan:
				switch i := e.(type) {
				case *NewTickEvent:
					tickEventsN ++
					totalTicks ++
					if hist, ok := histResponses[i.Ticker]; ok {
						assert.False(t, i.Tick.Datetime.Before(hist[len(hist)-1].Datetime))
					} else {
						t.Errorf("Got NewTickEvent without history response for %v", i.Ticker)
					}
				case *TickHistoryEvent:
					histEventsN ++
					_, ok := histResponses[i.Ticker]
					assert.False(t, ok)
					histResponses[i.Ticker] = i.Ticks
					totalTicks += len(i.Ticks)
					for n, tick := range i.Ticks {
						assert.Equal(t, tick.Ticker, i.Ticker)
						if n == 0 {
							continue
						} else {
							assert.False(t, tick.Datetime.Before(i.Ticks[n-1].Datetime))
						}
					}

				case *EndOfDataEvent:
					break HIST_LOOP
				}
			}
		}

		assert.True(t, tickEventsN > 0)
		assert.True(t, histEventsN > 0)
		assert.Equal(t, 1147, totalTicks)
	}*/

}
