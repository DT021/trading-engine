package engine

import (
	"alex/marketdata"
	"errors"
)

type errEndOfData struct {
	symbol string
}

func (e errEndOfData) Error() string {
	return e.symbol
}

type errNotEnoughtData struct {
	symbol string
}

func (e errNotEnoughtData) Error() string {
	return e.symbol
}



type OrderArray []*Order

type PositionArray []*Trade

type TradingContext struct {
	Symbol       string
	OpenOrders   OrderArray
	OpenPosition Trade
	LastCandles  marketdata.CandleArray
}

func (tc *TradingContext) Candle(index int) *marketdata.Candle {
	return tc.LastCandles[len(tc.LastCandles)-index]
}

type SignalProvider interface {
	OnBarUpdate(context *TradingContext) error
}

type Backtest struct {
	Symbol          string
	Candles         marketdata.CandleArray
	Logic           SignalProvider
	Context         TradingContext
	BarsBack        int
	lastInd         int
	CanceledOrders  OrderArray
	FilledOrders    OrderArray
	ClosedPositions PositionArray
}

func (bt *Backtest) checkExecutions() error {
	if len(bt.Context.OpenOrders) == 0 {
		return nil
	}

	if len(bt.Context.OpenOrders) > 1 {
		return errors.New("Too many orders.")
	}

	o := bt.Context.OpenOrders[0]

	if o.Type == "Market" {
		bt.Context.OpenOrders = OrderArray{}
		if bt.Context.OpenPosition.State == "New" {
			bt.Context.OpenPosition.Symbol = bt.Symbol
			bt.Context.OpenPosition.OpenTime = bt.Context.Candle(0).Datetime
			bt.Context.OpenPosition.OpenPrice = bt.Context.Candle(0).Open
			bt.Context.OpenPosition.State = "Open"
			bt.Context.OpenPosition.Qty = o.Qty
			o.State = "Filled"
			bt.FilledOrders = append(bt.FilledOrders, o)

			return nil

		} else {
			if bt.Context.OpenPosition.State != "Open" {
				return errors.New("Trade is closed")
			}
			if bt.Context.OpenPosition.Qty != - o.Qty {
				return errors.New("Wrong qty to close position")
			}

			o.State = "Filled"
			bt.Context.OpenPosition.CloseTime = bt.Context.Candle(0).Datetime
			bt.Context.OpenPosition.ClosePrice = bt.Context.Candle(0).Open
			bt.Context.OpenPosition.State = "Closed"

			bt.ClosedPositions = append(bt.ClosedPositions, &bt.Context.OpenPosition)

			bt.Context.OpenPosition = Trade{State: "New"}
		}

	}

	/*if o.Side == "B" {
		if o.Type == "Limit" {
			if candle.Low < o.Price {
				if candle.Open < o.Price {
					return candle.Open, true
				}

				return o.Price, true
			}
			return math.NaN(), false
		}

		if o.Type == "LOC" {
			if candle.Datetime.Hour() != 16 {
				return math.NaN(), false
			} else {
				if candle.Close < o.Price {
					return candle.Close, true
				}
				return math.NaN(), true
			}

		}

		if o.Type == "LOO" {
			if candle.Datetime.Hour() == 9 && candle.Datetime.Minute() == 30

		}
	} else {

	}*/
	return nil
}

func (bt *Backtest) pullData() error {
	bt.lastInd ++
	if bt.lastInd > len(bt.Candles)-1 {
		return &errEndOfData{bt.Symbol}
	}
	bt.Context.LastCandles = bt.Candles[bt.lastInd-bt.BarsBack : bt.lastInd]
	return nil
}

func (bt *Backtest) Run() error {
	if bt.BarsBack >= len(bt.Candles) {
		return errNotEnoughtData{bt.Symbol}
	}
	bt.lastInd = bt.BarsBack
	initialContext := TradingContext{
		bt.Symbol,
		OrderArray{},
		Trade{},
		marketdata.CandleArray{},
	}
	bt.Context = initialContext

BACKTEST_LOOP:
	for {

		err := bt.pullData()
		if err != nil {
			switch err.(type) {
			case errEndOfData:
				break BACKTEST_LOOP
			default:
				return err
			}
		}

		err = bt.checkExecutions()
		if err != nil {
			return err
		}

		bt.Logic.OnBarUpdate(&bt.Context)
	}

	return nil
}
