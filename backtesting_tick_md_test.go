package engine

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"fmt"
)

func TestBacktestingTickMarketdata_getFilename(t *testing.T) {
	m := BacktestingTickMarketdata{}

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
