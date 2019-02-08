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

type BacktestingTickMarketdata struct {
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
}

func (m *BacktestingTickMarketdata) getFilename() (string, error) {
	if len(m.Symbols) == 0 {
		return "", errors.New("Symbols len is zero")
	}
	sort.Strings(m.Symbols)
	out := ""
	for _, v := range m.Symbols {
		out += v + ","
	}
	if m.LoadTicks{
		out +="LoadTicks,"
	}
	if m.LoadQuotes{
		out += "LoadQuotes,"
	}
	//todo add dates here

	h := fnv.New32a()
	h.Write([]byte(out))
	return strconv.FormatUint(uint64(h.Sum32()), 10), nil

}

func (m *BacktestingTickMarketdata) prepare() {
	d:= m.FromDate
	for{
		m.loadDate(d)
		d = d.AddDate(0,0, 1)
		if d.After(m.ToDate){
			break
		}
	}
}

func (m *BacktestingTickMarketdata) loadDate(date time.Time) {
	totalTicks := marketdata.TickArray{}
	for _, symbol := range m.Symbols {
		rng := marketdata.DateRange{
			From:time.Date(date.Year(), date.Month(), date.Minute(), 0, 0, 0, 0, time.UTC),
			To:time.Date(date.Year(), date.Month(), date.Minute(), 23, 59, 59, 59, time.UTC),
		}
		symbolTicks, err := m.Storage.GetStoredTicks(symbol, rng, m.LoadQuotes, m.LoadTicks)
		if err != nil {
			go m.newError(err)
			continue
		}

		totalTicks = append(totalTicks, symbolTicks...)
		totalTicks = totalTicks.Sort()
		m.writeDateTicks(totalTicks)
	}
}

func (m *BacktestingTickMarketdata) writeDateTicks(ticks marketdata.TickArray) {

}

func (m *BacktestingTickMarketdata) newError(err error) {
	m.errChan <- err
}

func (m *BacktestingTickMarketdata) prepairedDataExists() bool {
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

func (m *BacktestingTickMarketdata) Run() {
	if !m.prepairedDataExists() {
		m.prepare()
	}

	go m.genTickEvents()

}

func (m *BacktestingTickMarketdata) genTickEvents() {
	if !m.prepairedDataExists() {
		panic("Can't genereate tick events. Prepaired data is not exists. ")
	}
	//Todo

}

func (m *BacktestingTickMarketdata) Connect(errChan chan error, eventChan chan event) {
	if errChan == nil {
		panic("Error chan is nil")
	}

	if eventChan == nil {
		panic("Event chan is nil")
	}

	m.eventChan = eventChan
	m.errChan = errChan
}
