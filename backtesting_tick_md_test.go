package engine

import (
	"alex/marketdata"
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBTM_getFilename(t *testing.T) {
	m := BTM{}

	symbols1 := []string{"S1", "S2", "S#"}
	symbols2 := []string{"S2", "S1", "S#"}
	symbols3 := []string{"S1", "S4", "S#"}

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

type mockStorage struct {
	folder string
}

func (s *mockStorage) GetStoredTicks(symbol string, dRange marketdata.DateRange, quotes bool, trades bool) (marketdata.TickArray, error) {
	if dRange.From.Weekday() != dRange.To.Weekday() {
		panic("mockStorage can work only with single date in datarange")
	}
	filename := dRange.To.Format("2006-01-02") + ".json"
	pth := path.Join(s.folder, symbol, filename)
	if _, err := os.Stat(pth); os.IsNotExist(err) {
		return nil, err
	}

	jsonFile, err := os.Open(pth)

	if err != nil {
		return nil, err
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var ticks marketdata.TickArray

	err = json.Unmarshal(byteValue, &ticks)

	if err != nil {
		return nil, err
	}

	for _, t := range ticks {
		t.Symbol = symbol
	}

	return ticks, err

}

func (s *mockStorage) GetStoredCandles(symbol string, tf string, dRange marketdata.DateRange) (*marketdata.CandleArray, error) {
	return nil, errors.New("Not implemented method for mockStorage")
}

func createDirIfNotExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {

		err := os.MkdirAll(dirPath, os.ModePerm)
		return err
	}

	return nil
}

func newTestBTM() *BTM {
	testSymbols := []string{
		"Sym1",
		"Sym2",
		"Sym3",
		"Sym4",
		"Sym5",
		"Sym6",
		"Sym7",
	}
	fromDate := time.Date(2018, 3, 2, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2018, 3, 10, 0, 0, 0, 0, time.UTC)
	storage := mockStorage{folder: "./test_data/json_storage/ticks/quotes_trades"}
	b := BTM{
		Symbols:    testSymbols,
		Folder:     "./test_data/BTM",
		LoadQuotes: true,
		LoadTicks:  true,
		FromDate:   fromDate,
		ToDate:     toDate,
		Storage:    &storage,
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
	defer file.Close()

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

	b := newTestBTM()
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

func assertNoEventsGeneratedByBTM(t *testing.T, b *BTM) {
	select {
	case v, ok := <-b.mdChan:
		assert.False(t, ok)
		if ok {
			t.Errorf("ERROR! Expected no events. Found: %v", v)
		}
	default:
		t.Log("OK! Events chan is empty")
		break
	}
}

func TestBTM_Run(t *testing.T) {
	b := newTestBTM()

	err := os.Remove(b.getPrepairedFilePath())
	if err != nil {
		t.Error(err)
	}
	b.Run()
	time.Sleep(10 * time.Millisecond)
	totalE := 0
	var prevTime time.Time

LOOP:
	for {
		select {
		case e := <-b.mdChan:
			i := e.(event)
			totalE += 1
			assert.False(t, i.getTime().Before(prevTime))
			prevTime = i.getTime()
			if totalE == 1146 {
				t.Log("OK! Found last event")
				break LOOP
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Not found enough ticks")

		}
	}

	e := <-b.mdChan
	assert.NotNil(t, e)
	assert.IsType(t, &EndOfDataEvent{}, e)

}
