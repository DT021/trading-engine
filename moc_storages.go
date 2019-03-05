package engine

import (
	"alex/marketdata"
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path"
)

type mockStorageJSON struct {
	folder string
}
func (s *mockStorageJSON) GetStoredTicks(symbol string, dRange marketdata.DateRange, quotes bool, trades bool) (marketdata.TickArray, error) {
	if dRange.From.Weekday() != dRange.To.Weekday() {
		panic("mockStorageJSON can work only with single date in datarange")
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

func (s *mockStorageJSON) GetStoredCandles(symbol string, tf string, dRange marketdata.DateRange) (marketdata.CandleArray, error) {
	return nil, errors.New("Not implemented method for mockStorageJSON")
}



