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
	return strconv.FormatUint(uint64(h.Sum32()), 10)+".prep", nil

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
			From: time.Date(date.Year(), date.Month(), date.Minute(), 0, 0, 0, 0, time.UTC),
			To:   time.Date(date.Year(), date.Month(), date.Minute(), 23, 59, 59, 59, time.UTC),
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
	//Todo

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
