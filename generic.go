package engine

import (
	"time"
	"github.com/pkg/errors"
)

type OrderType string
type OrderSide string
type OrderState string
type TradeType string

const (
	OrderBuy OrderSide = "B"
	OrderSell OrderSide = "S"
	EmptyOrder     OrderState = "EmptyOrder"
	NewOrder       OrderState = "NewOrder"
	ConfirmedOrder OrderState = "ConfirmedOrder"
	FilledOrder    OrderState = "FilledOrder"
	CanceledOrder  OrderState = "CanceledOrder"

	LimitOrder    OrderType = "LMT"
	MarketOrder   OrderType = "MKT"
	StopOrder     OrderType = "STP"
	LimitOnClose  OrderType = "LOC"
	LimitOnOpen   OrderType = "LOO"
	MarketOnClose OrderType = "MOC"
	MarketOnOpen  OrderType = "MOO"

	FlatTrade   TradeType = "FlatTrade"
	LongTrade   TradeType = "LongTrade"
	ShortTrade  TradeType = "ShortTrade"
	ClosedTrade TradeType = "ClosedTrade"

)

type Order struct {
	Side   string
	Qty    int
	Symbol string
	State  OrderState
	Price  float64
	Type   OrderType
	Id     string
}

func NewEmptyOrder() {

}

func (o *Order) Created() bool {
	if o.Type != "" && o.Price != 0 {
		return true
	}
	return false
}

type Trade struct {
	Symbol          string
	Qty             int
	Type            TradeType
	OpenPrice       float64
	ClosePrice      float64
	OpenTime        time.Time
	CloseTime       time.Time
	Marks           string
	FilledOrders    []*Order
	CanceledOrders  []*Order
	NewOrders       []*Order
	ConfirmedOrders []*Order
	ClosedPnL       float64
	OpenPnL         float64
	Id string

}

func (t *Trade) putNewOrder(o *Order) error{
	if o.State!= NewOrder{
		return errors.New("Trying to add not new order")
	}
	t.NewOrders = append(t.NewOrders, o)
	return nil
}

func (t *Trade) confirmOrder(id string) error{
	return nil
}

func (t *Trade) update(){

}

func (t *Trade) AddExecution(order *Order) error {
	return nil
}

func (t *Trade) IsOpen() bool {
	if t.Qty != 0 {
		return true
	} else {
		return false
	}
}
