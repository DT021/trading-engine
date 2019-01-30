package engine

import (
	"gonum.org/v1/plot/plotter"
	"encoding/json"
	"io/ioutil"
	"fmt"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func PlotEquity(trades []Trade, savePath string) {
	pth := savePath + "_dol.png"
	savePlot(trades, pth, makePointsDollar)

	pth = savePath + "_per.png"
	savePlot(trades, pth, makePointsPercent)

}

type pointCreator func(n []Trade) plotter.XYs

func savePlot(trades []Trade, savePath string, f pointCreator) {
	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	p.Title.Text = "Backtest Results"
	p.X.Label.Text = "X"
	p.Y.Label.Text = "Y"

	err = plotutil.AddLinePoints(p,
		"First", f(trades))
	if err != nil {
		panic(err)
	}

	if err := p.Save(7*vg.Inch, 7*vg.Inch, savePath); err != nil {
		panic(err)
	}
}

func makePointsDollar(n []Trade) plotter.XYs {
	pts := make(plotter.XYs, len(n))
	equity := 0.0
	for i := range pts {
		equity += n[i].ClosedPnL
		pts[i].X = float64(i)
		pts[i].Y = equity
	}
	return pts
}

func makePointsPercent(n []Trade) plotter.XYs {
	pts := make(plotter.XYs, len(n))
	equity := 0.0
	for i := range pts {
		equity += n[i].ClosedPnL * 100 / n[i].FirstPrice
		pts[i].X = float64(i)
		pts[i].Y = equity
	}
	return pts
}

func SaveTrades(t []Trade, savePath string) error {

	json_, err := json.Marshal(&t)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(savePath, json_, 0644)
	if err != nil {
		fmt.Println(fmt.Sprintf("Can't write candles To file: %v", err))
	}

	return err
}
