package engine

import (
	"sort"
	"hash/fnv"
	"github.com/pkg/errors"
	"strconv"
	"path"
	"os"
	"alex/marketdata"
	"time"
	"bufio"
	"strings"
)

type IMarketData interface {
	Done() bool
	Next() *event
	Pop() *event
	Run()
}

type ITimerEventProducer interface {
	Next()
	Pop() *event
}

type BTM struct {
	Symbols          []string
	Folder           string
	LoadQuotes       bool
	LoadTicks        bool
	FromDate         time.Time
	ToDate           time.Time
	UsePrepairedData bool
	errChan          chan error
	eventChan        chan event
	Storage          marketdata.Storage
	fraction         int64
}

func (m *BTM) getFilename() (string, error) {
	if len(m.Symbols) == 0 {
		return "", errors.New("Symbols len is zero")
	}
	sort.Strings(m.Symbols)
	out := ""
	for _, v := range m.Symbols {
		out += v + ","
	}
	if m.LoadTicks {
		out += "LoadTicks,"
	}
	if m.LoadQuotes {
		out += "LoadQuotes,"
	}

	datesToStringLayout := "2006-01-02 15:04:05"
	out += m.FromDate.Format(datesToStringLayout) + "," + m.ToDate.Format(datesToStringLayout)

	h := fnv.New32a()
	h.Write([]byte(out))
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
	d := m.FromDate
	if m.prepairedDataExists() {
		err := m.clearPrepairedData()
		if err != nil {
			panic(err)
		}
	}

	for {
		m.loadDate(d)
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

func (m *BTM) loadDate(date time.Time) {
	totalTicks := marketdata.TickArray{}
	for _, symbol := range m.Symbols {
		rng := marketdata.DateRange{
			From: time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC),
			To:   time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 59, time.UTC),
		}
		symbolTicks, err := m.Storage.GetStoredTicks(symbol, rng, m.LoadQuotes, m.LoadTicks)
		if err != nil && symbolTicks != nil {
			go m.newError(err)
			continue
		}

		totalTicks = append(totalTicks, symbolTicks...)

	}
	totalTicks = totalTicks.Sort()
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

	defer f.Close()

	for _, t := range ticks {
		if _, err := f.Write([]byte(t.String() + "\n")); err != nil {
			go m.newError(err)
		}
	}
}

func (m *BTM) newError(err error) {
	m.errChan <- err
}

func (m *BTM) newEvent(e event) {
	if m.eventChan == nil {
		panic("BTM event chan is nil")
	}
	m.eventChan <- e
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

	go m.genTickEvents()

}

func (m *BTM) genTickEvents() {
	if !m.prepairedDataExists() {
		panic("Can't genereate tick events. Prepaired data is not exists. ")
	}

	file, err := os.Open(m.getPrepairedFilePath())
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var prevTick *marketdata.Tick
	for scanner.Scan() {
		tick, err := m.parseLineToTick(scanner.Text())
		if err != nil {
			panic(err)
		}
		if prevTick == nil {
			prevTick = tick
			continue
		}

		delta := tick.Datetime.Sub(prevTick.Datetime)
		e := NewTickEvent{
			Tick: tick,
			Time: tick.Datetime,
		}

		if delta.Seconds() < 2 {
			time.Sleep(time.Duration(delta.Nanoseconds()/m.fraction) * time.Nanosecond)
		} else {
			time.Sleep(time.Duration(2/m.fraction) * time.Second)
		}

		m.newEvent(&e)

		prevTick = tick

	}

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

func (m *BTM) Connect(errChan chan error, eventChan chan event) {
	if errChan == nil {
		panic("Error chan is nil")
	}

	if eventChan == nil {
		panic("Event chan is nil")
	}

	m.eventChan = eventChan
	m.errChan = errChan
}
