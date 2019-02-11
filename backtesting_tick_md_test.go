package engine

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"fmt"
	"alex/marketdata"
	"github.com/pkg/errors"
	"path"
	"os"
	"io/ioutil"
	"encoding/json"
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
	toDate := time.Date(2018, 10, 2, 0, 0, 0, 0, time.UTC)
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

	createDirIfNotExists(b.Folder)

	return &b
}

func TestBTM_prepare(t *testing.T) {
	b := newTestBTM()
	n, err := b.getFilename()
	if err != nil {
		t.Error(err)
	}
	t.Log(n)
}
