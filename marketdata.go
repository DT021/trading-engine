package engine

import (
	"alex/marketdata"
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"hash/fnv"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MarketDataMode string

const (
	MarketDataModeTicks       MarketDataMode = "MarketDataModeTicks"
	MarketDataModeTicksQuotes MarketDataMode = "MarketDataModeTicksQuotes"
	MarketDataModeQuotes      MarketDataMode = "MarketDataModeQuotes"
	MarketDataModeCandles     MarketDataMode = "MarketDataModeCandles"
)

type IMarketData interface {
	Run()
	Connect()
	Init(errChan chan error, mdChan chan event)
	SetSymbols(symbols []*Instrument)
	RequestHistoricalData(duration time.Duration)
	ShutDown()
}

type BTM struct {
	Symbols          []*Instrument
	Folder           string
	FromDate         time.Time
	ToDate           time.Time
	UsePrepairedData bool
	candlesTimeFrame string

	errChan          chan error
	mdChan           chan event
	Storage          marketdata.Storage
	histDataTimeBack time.Duration
	waitGroup        *sync.WaitGroup
	mode             MarketDataMode
}

func (m *BTM) ShutDown() {
	m.waitGroup.Wait()
}

func (m *BTM) SetSymbols(symbols []*Instrument) {
	m.Symbols = symbols
}

func (m *BTM) Connect() {
	fmt.Println("Backtest market data connected. ")
}

func (m *BTM) Init(errChan chan error, mdChan chan event) {
	if errChan == nil {
		panic("Error chan is nil")
	}

	if mdChan == nil {
		panic("Event chan is nil")
	}

	m.mdChan = mdChan
	m.errChan = errChan
	m.histDataTimeBack = time.Duration(0) * time.Second
}

func (m *BTM) RequestHistoricalData(duration time.Duration) {
	m.histDataTimeBack = duration
}

func (m *BTM) getFilename() (string, error) {
	if len(m.Symbols) == 0 {
		return "", errors.New("symbols len is zero")
	}
	rawSymbols := make([]string, len(m.Symbols))
	for i, s := range m.Symbols {
		rawSymbols[i] = s.Symbol
	}
	sort.Strings(rawSymbols)
	out := ""
	for _, v := range rawSymbols {
		out += v + ","
	}

	out += string(m.mode)
	if m.mode == MarketDataModeCandles {
		out += m.candlesTimeFrame
	}

	datesToStringLayout := "2006-01-02 15:04:05"
	out += m.FromDate.Format(datesToStringLayout) + "," + m.ToDate.Format(datesToStringLayout)

	h := fnv.New32a()
	_, err := h.Write([]byte(out))
	if err != nil {
		panic(err)
	}
	return strconv.FormatUint(uint64(h.Sum32()), 10) + ".prep", nil
}

func (m *BTM) getPrepairedFilePath() string {
	fpth, err := m.getFilename()
	if err != nil {
		panic(err)
	}
	return path.Join(m.Folder, fpth)
}

func (m *BTM) prepare() {
	if m.prepairedDataExists() {
		err := m.clearPrepairedData()
		if err != nil {
			panic(err)
		}
	}

	if m.mode == MarketDataModeTicksQuotes || m.mode == MarketDataModeTicks || m.mode == MarketDataModeQuotes {
		m.prepareTicks()
		return
	}
	if m.mode == MarketDataModeCandles {
		m.prepareCandles()
		return
	}

	panic("Unknown market data mode: " + string(m.mode))

}

func (m *BTM) prepareCandles() {
	rng := marketdata.DateRange{
		From: m.FromDate,
		To:   m.ToDate,
	}
	var totalcandles marketdata.CandleArray
	for _, s := range m.Symbols {
		sc, err := m.Storage.GetStoredCandles(s.Symbol, m.candlesTimeFrame, rng)
		if err != nil {
			m.newError(err)
		}
		if sc != nil {
			totalcandles = append(totalcandles, sc...)
		}
	}

	if len(totalcandles) == 0 {
		panic("No candles were loaded")
	}

	totalcandles.Sort()
	m.writeCandles(totalcandles)

}

func (m *BTM) prepareTicks() {
	d := m.FromDate
	for {
		m.loadDateTicks(d)
		d = d.AddDate(0, 0, 1)
		if d.After(m.ToDate) {
			break
		}
	}
}

func (m *BTM) clearPrepairedData() error {
	if !m.prepairedDataExists() {
		return nil
	}

	err := os.Remove(m.getPrepairedFilePath())
	return err
}

func (m *BTM) loadDateTicks(date time.Time) {
	totalTicks := marketdata.TickArray{}
	for _, symbol := range m.Symbols {
		rng := marketdata.DateRange{
			From: time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC),
			To:   time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 59, time.UTC),
		}
		loadQuotes := false
		loadTicks := false
		if m.mode == MarketDataModeTicks || m.mode == MarketDataModeTicksQuotes {
			loadTicks = true
		}
		if m.mode == MarketDataModeTicksQuotes || m.mode == MarketDataModeQuotes {
			loadQuotes = true
		}
		symbolTicks, err := m.Storage.GetStoredTicks(symbol.Symbol, rng, loadQuotes, loadTicks)
		if err != nil && symbolTicks != nil {
			m.newError(err)
			continue
		}

		totalTicks = append(totalTicks, symbolTicks...)

	}
	totalTicks.Sort()
	m.writeDateTicks(totalTicks)
}

func (m *BTM) writeDateTicks(ticks marketdata.TickArray) {
	if len(ticks) == 0 {
		return
	}

	f, err := os.OpenFile(m.getPrepairedFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		panic(err)
	}

	defer func() {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}()

	for _, t := range ticks {
		if !t.HasTrade() {
			continue
		}
		if _, err := f.Write([]byte(t.String() + "\n")); err != nil {
			m.newError(err)
		}
	}
}

func (m *BTM) writeCandles(candles marketdata.CandleArray) {
	if len(candles) == 0 {
		return
	}

	f, err := os.OpenFile(m.getPrepairedFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		panic(err)
	}

	defer func() {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}()

	for _, t := range candles {
		if _, err := f.Write([]byte(t.String() + "\n")); err != nil {
			m.newError(err)
		}
	}

}

func (m *BTM) newError(err error) {
	m.waitGroup.Add(1)
	go func() {
		m.errChan <- err
		m.waitGroup.Done()
	}()

}

func (m *BTM) newEvent(e event) {
	if m.mdChan == nil {
		panic("BTM event chan is nil")
	}
	m.mdChan <- e
}

func (m *BTM) prepairedDataExists() bool {
	filename, err := m.getFilename()
	if err != nil {
		panic(err)
	}
	pth := path.Join(m.Folder, filename)

	if _, err := os.Stat(pth); os.IsNotExist(err) {
		return false
	}

	return true
}

func (m *BTM) Run() {
	if !m.prepairedDataExists() {
		m.prepare()
	}
	if m.mode == MarketDataModeQuotes || m.mode == MarketDataModeTicks || m.mode == MarketDataModeTicksQuotes {
		if m.histDataTimeBack > time.Second {
			m.waitGroup.Add(1)
			go func() {
				m.genTickEventsWithHistory()
				m.waitGroup.Done()
			}()

		} else {
			m.waitGroup.Add(1)
			go func() {
				m.genTickEvents()
				m.waitGroup.Done()
			}()
		}

		return
	}

	if m.mode == MarketDataModeCandles {
		if m.histDataTimeBack > time.Minute {
			m.waitGroup.Add(1)
			go func() {
				m.genCandlesEventsWithHistory()
				m.waitGroup.Done()
			}()

		} else {
			m.waitGroup.Add(1)
			go func() {
				m.genCandlesEvents()
				m.waitGroup.Done()
			}()
		}

		return
	}

	panic("Unknown market data mode")

}

func (m *BTM) getTickersMap() map[string]*Instrument {
	tickersMap := make(map[string]*Instrument)

	for _, s := range m.Symbols {
		tickersMap[s.Symbol] = s
	}

	return tickersMap
}

func (m *BTM) genTickEvents() {
	if !m.prepairedDataExists() {
		panic("Can't genereate tick events. Prepaired data is not exists. ")
	}

	file, err := os.Open(m.getPrepairedFilePath())
	if err != nil {
		panic(err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			m.newError(err)
		}
	}()

	scanner := bufio.NewScanner(file)
	tickersMap := m.getTickersMap()

	for scanner.Scan() {
		tickRaw, err := m.parseLineToTick(scanner.Text())
		if err != nil {
			panic(err)
		}

		ticker := tickersMap[tickRaw.Symbol]
		tick := Tick{
			Tick:   tickRaw,
			Ticker: ticker,
		}

		e := NewTickEvent{
			Tick:      &tick,
			BaseEvent: BaseEvent{Time: tick.Datetime, Ticker: ticker},
		}

		m.newEvent(&e)

	}
	m.newEvent(&EndOfDataEvent{BaseEvent: be(time.Now(), &Instrument{})})

}

func (m *BTM) genTickEventsWithHistory() {
	if !m.prepairedDataExists() {
		panic("Can't genereate tick events. Prepaired data is not exists. ")
	}

	file, err := os.Open(m.getPrepairedFilePath())
	if err != nil {
		panic(err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			m.newError(err)
		}
	}()

	historyMap := make(map[string]TickArray)
	historyLoaded := make(map[string]struct{})

	scanner := bufio.NewScanner(file)
	tickersMap := m.getTickersMap()

	for scanner.Scan() {
		tickRaw, err := m.parseLineToTick(scanner.Text())
		if err != nil {
			panic(err)
		}
		ticker := tickersMap[tickRaw.Symbol]
		tick := Tick{
			Tick:   tickRaw,
			Ticker: ticker,
		}

		//Put new tick event if we already got all history
		if _, ok := historyLoaded[tick.Symbol]; ok {
			e := NewTickEvent{
				Tick:      &tick,
				BaseEvent: BaseEvent{Time: tick.Datetime, Ticker: ticker},
			}
			m.newEvent(&e)
			continue
		}

		//If we have in history map something - check timespan. If history is full put history events
		if arr, ok := historyMap[tick.Symbol]; ok {

			delta := tick.Datetime.Sub(arr[0].Datetime)
			if delta < m.histDataTimeBack {
				historyMap[tick.Symbol] = append(historyMap[tick.Symbol], &tick)

			} else {
				//Than put in chan history resp event
				historyLoaded[tick.Symbol] = struct{}{}
				historyEvent := TickHistoryEvent{
					BaseEvent: be(arr[len(arr)-1].Datetime, ticker),
					Ticks:     historyMap[tick.Symbol],
				}
				m.newEvent(&historyEvent)

				//First put out new tick event
				e := NewTickEvent{
					Tick:      &tick,
					BaseEvent: BaseEvent{Time: tick.Datetime, Ticker: ticker},
				}
				m.newEvent(&e)

			}
		} else {
			historyMap[tick.Symbol] = TickArray{&tick}

		}

	}
	m.newEvent(&EndOfDataEvent{BaseEvent: be(time.Now(), &Instrument{})})

}

func (m *BTM) genCandlesEvents() {
	if !m.prepairedDataExists() {
		panic("Can't genereate tick events. Prepaired data is not exists. ")
	}

	file, err := os.Open(m.getPrepairedFilePath())
	if err != nil {
		panic(err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			m.newError(err)
		}
	}()

	scanner := bufio.NewScanner(file)
	var candleCloses []*CandleCloseEvent
	tickersMap := m.getTickersMap()

	for scanner.Scan() {
		cRaw, err := m.parseLineToCandle(scanner.Text())
		if err != nil {
			panic(err)
		}

		ticker := tickersMap[cRaw.Symbol]
		e := CandleOpenEvent{
			BaseEvent:  be(cRaw.Datetime, ticker),
			CandleTime: cRaw.Datetime,
			Price:      cRaw.Open,
			TimeFrame:  m.candlesTimeFrame,
		}

		//If we have stored events and current candle open event is not before first listed candle close event
		if len(candleCloses) > 0 && !candleCloses[0].getTime().After(e.getTime()) {
			for _, pce := range candleCloses {
				m.newEvent(pce)
			}
			candleCloses = []*CandleCloseEvent{}
		}

		m.newEvent(&e)

		c := Candle{
			Candle: cRaw,
			Ticker: ticker,
		}

		ce := CandleCloseEvent{
			BaseEvent: be(cRaw.Datetime, ticker),
			Candle:    &c,
			TimeFrame: m.candlesTimeFrame,
		}
		ce.setEventTimeFromCandle()
		candleCloses = append(candleCloses, &ce)

	}

	if len(candleCloses) > 0 {
		for _, pce := range candleCloses {
			m.newEvent(pce)
		}
	}
	m.newEvent(&EndOfDataEvent{BaseEvent: be(time.Now(), &Instrument{})})

}

func (m *BTM) genCandlesEventsWithHistory() {
	if !m.prepairedDataExists() {
		panic("Can't genereate tick events. Prepaired data is not exists. ")
	}

	file, err := os.Open(m.getPrepairedFilePath())
	if err != nil {
		panic(err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			m.newError(err)
		}
	}()

	scanner := bufio.NewScanner(file)

	historyMap := make(map[string]CandleArray)
	historyLoaded := make(map[string]struct{})

	var candleCloses []*CandleCloseEvent
	tickersMap := m.getTickersMap()

	for scanner.Scan() {
		cRaw, err := m.parseLineToCandle(scanner.Text())
		if err != nil {
			panic(err)
		}

		ticker := tickersMap[cRaw.Symbol]

		c := &Candle{
			Candle: cRaw,
			Ticker: ticker,
		}

		if _, ok := historyLoaded[cRaw.Symbol]; ok {
			e := CandleOpenEvent{
				BaseEvent:  be(cRaw.Datetime, ticker),
				CandleTime: cRaw.Datetime,
				Price:      cRaw.Open,
				TimeFrame:  m.candlesTimeFrame,
			}

			//If we have stored events and current candle open event is not before first listed candle close event
			if len(candleCloses) > 0 && !candleCloses[0].getTime().After(e.getTime()) {
				for _, pce := range candleCloses {
					m.newEvent(pce)
				}
				candleCloses = []*CandleCloseEvent{}
			}

			m.newEvent(&e)

			ce := CandleCloseEvent{
				BaseEvent: be(cRaw.Datetime, ticker),
				Candle:    c,
				TimeFrame: m.candlesTimeFrame,
			}
			ce.setEventTimeFromCandle()
			candleCloses = append(candleCloses, &ce)
			continue
		}

		if arr, ok := historyMap[cRaw.Symbol]; ok {

			delta := cRaw.Datetime.Sub(arr[0].Datetime)
			if delta < m.histDataTimeBack {
				historyMap[cRaw.Symbol] = append(historyMap[cRaw.Symbol], c)

			} else {
				//Than put in chan history resp event
				historyLoaded[cRaw.Symbol] = struct{}{}
				historyEvent := CandlesHistoryEvent{
					BaseEvent: be(arr[len(arr)-1].Datetime, ticker),
					Candles:   historyMap[cRaw.Symbol],
				}
				m.newEvent(&historyEvent)

				//First put out new tick event
				e := CandleOpenEvent{
					BaseEvent:  be(cRaw.Datetime, ticker),
					CandleTime: cRaw.Datetime,
					Price:      cRaw.Open,
					TimeFrame:  m.candlesTimeFrame,
				}

				if len(candleCloses) > 0 && !candleCloses[0].getTime().After(e.getTime()) {
					for _, pce := range candleCloses {
						m.newEvent(pce)
					}
					candleCloses = []*CandleCloseEvent{}
				}

				m.newEvent(&e)

				ce := CandleCloseEvent{
					BaseEvent: be(cRaw.Datetime, ticker),
					Candle:    c,
					TimeFrame: m.candlesTimeFrame,
				}
				ce.setEventTimeFromCandle()
				candleCloses = append(candleCloses, &ce)

			}
		} else {
			historyMap[cRaw.Symbol] = CandleArray{c}

		}

	}

	if len(candleCloses) > 0 {
		for _, pce := range candleCloses {
			m.newEvent(pce)
		}
	}
	m.newEvent(&EndOfDataEvent{BaseEvent: be(time.Now(), &Instrument{})})

}

func (m *BTM) parseLineToTick(l string) (*marketdata.Tick, error) {
	lsp := strings.Split(l, ",")
	if len(lsp) != 16 {
		return nil, errors.New("Can't parse row: " + l)
	}

	last, err := strconv.ParseFloat(lsp[2], 64)
	if err != nil {
		return nil, err
	}

	lastSize, err := strconv.ParseInt(lsp[3], 10, 64)
	if err != nil {
		return nil, err
	}

	bid, err := strconv.ParseFloat(lsp[5], 64)
	if err != nil {
		return nil, err
	}

	bidSize, err := strconv.ParseInt(lsp[6], 10, 64)
	if err != nil {
		return nil, err
	}

	ask, err := strconv.ParseFloat(lsp[8], 64)
	if err != nil {
		return nil, err
	}

	askSize, err := strconv.ParseInt(lsp[9], 10, 64)
	if err != nil {
		return nil, err
	}

	i, err := strconv.ParseInt(lsp[0], 10, 64)
	if err != nil {
		return nil, err
	}
	tm := time.Unix(i, 0)

	tick := marketdata.Tick{
		Datetime:  tm,
		Symbol:    lsp[1],
		LastPrice: last,
		LastSize:  lastSize,
		LastExch:  lsp[4],
		BidPrice:  bid,
		BidSize:   bidSize,
		BidExch:   lsp[7],
		AskPrice:  ask,
		AskSize:   askSize,
		AskExch:   lsp[10],
		CondQuote: lsp[11],
		Cond1:     lsp[12],
		Cond2:     lsp[13],
		Cond3:     lsp[14],
		Cond4:     lsp[15],
	}

	return &tick, nil
}

func (m *BTM) parseLineToCandle(l string) (*marketdata.Candle, error) {
	ls := strings.Split(l, ",")
	if len(ls) != 9 {
		return nil, errors.New("Can't parse line to candle: " + l)
	}

	i, err := strconv.ParseInt(ls[0], 10, 64)
	if err != nil {
		return nil, err
	}
	tm := time.Unix(i, 0)

	symbol := ls[1]

	open, err := strconv.ParseFloat(ls[2], 64)
	if err != nil {
		return nil, err
	}

	high, err := strconv.ParseFloat(ls[3], 64)
	if err != nil {
		return nil, err
	}

	low, err := strconv.ParseFloat(ls[4], 64)
	if err != nil {
		return nil, err
	}

	close_, err := strconv.ParseFloat(ls[5], 64)
	if err != nil {
		return nil, err
	}

	closeAdj, err := strconv.ParseFloat(ls[6], 64)
	if err != nil {
		return nil, err
	}

	volume, err := strconv.ParseInt(ls[7], 10, 64)
	if err != nil {
		return nil, err
	}

	openInterest, err := strconv.ParseInt(ls[8], 10, 64)
	if err != nil {
		return nil, err
	}

	c := marketdata.Candle{
		Datetime:     tm,
		Symbol:       symbol,
		Open:         open,
		High:         high,
		Low:          low,
		Close:        close_,
		AdjClose:     closeAdj,
		Volume:       volume,
		OpenInterest: openInterest,
	}

	return &c, nil
}
