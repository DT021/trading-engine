package engine

import (
	"time"
	"github.com/pkg/errors"
	"math"
)

type OrderType string
type OrderSide string
type OrderState string
type TradeType string

const (
	OrderBuy           OrderSide  = "B"
	OrderSell          OrderSide  = "S"
	EmptyOrder         OrderState = "EmptyOrder"
	NewOrder           OrderState = "NewOrder"
	ConfirmedOrder     OrderState = "ConfirmedOrder"
	FilledOrder        OrderState = "FilledOrder"
	PartialFilledOrder OrderState = "PartialFilledOrder"
	CanceledOrder      OrderState = "CanceledOrder"

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
	Side      OrderSide
	Qty       int
	ExecQty   int
	Symbol    string
	State     OrderState
	Price     float64
	ExecPrice float64
	Type      OrderType
	Id        string
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
	Symbol      string
	Qty         int
	Type        TradeType
	OpenPrice   float64
	OpenValue   float64
	MarketValue float64

	OpenTime        time.Time
	CloseTime       time.Time
	Marks           string
	FilledOrders    map[string]*Order
	CanceledOrders  map[string]*Order
	NewOrders       map[string]*Order
	ConfirmedOrders map[string]*Order
	RejectedOrders  map[string]*Order
	AllOrdersIDMap  map[string]struct{}
	ClosedPnL       float64
	OpenPnL         float64
	Id              string
}

func newEmptyTrade(symbol string) *Trade {
	trade := Trade{Symbol: symbol, Qty: 0, Type: FlatTrade, OpenPrice: math.NaN(),
		ClosedPnL: 0, OpenPnL: 0, FilledOrders: make(map[string]*Order), CanceledOrders: make(map[string]*Order),
		NewOrders: make(map[string]*Order), ConfirmedOrders: make(map[string]*Order), RejectedOrders: make(map[string]*Order),
		AllOrdersIDMap: make(map[string]struct{})}

	return &trade
}

//putNewOrder inserts order in NewOrders map. If there are order with same id in all orders
//map it will return error. There can't few orders even in different states with the same id
func (t *Trade) putNewOrder(o *Order) error {
	if o.Symbol != t.Symbol {
		return errors.New("Can't put new order. Trade and Order have different symbols")
	}
	if o.State != NewOrder {
		return errors.New("Trying to add not new order")
	}

	if len(t.NewOrders) == 0 {
		t.NewOrders = make(map[string]*Order)
	} else {
		if _, ok := t.NewOrders[o.Id]; ok {
			return errors.New("Trying to add order in NewOrders with the ID that already in map")
		}
	}

	if len(t.AllOrdersIDMap) == 0 {
		t.AllOrdersIDMap = make(map[string]struct{})
	} else {
		if _, ok := t.AllOrdersIDMap[o.Id]; ok {
			return errors.New("Order with this ID is already in Trade orders maps")
		}
	}

	t.NewOrders[o.Id] = o
	t.AllOrdersIDMap[o.Id] = struct{}{}
	return nil
}

//confirmOrder confirms order by ID if it's in NewOrder map. Order moves from NewOrder map to ConfirmedOrders map
//it returns error if ID not in NewOrders map and if ID is already in ConfirmedOrders map
func (t *Trade) confirmOrder(id string) error {
	if order, ok := t.NewOrders[id]; !ok {
		return errors.New("Can't confirm order. ID is not in the NewOrders map")
	} else {
		if _, ok := t.ConfirmedOrders[id]; ok {
			return errors.New("Can't confirm orders. ID already in ConfirmedOrders map")
		}

		order.State = ConfirmedOrder
		if len(t.ConfirmedOrders) == 0 {
			t.ConfirmedOrders = make(map[string]*Order)
		}
		t.ConfirmedOrders[id] = order
		delete(t.NewOrders, id)
	}

	return nil
}

//cancelOrder removes order from confirmed list and puts it to cancel list. If there are no order with
//specified id it will return error.
func (t *Trade) cancelOrder(id string) error {
	if order, ok := t.ConfirmedOrders[id]; ok {
		order.State = CanceledOrder
		if len(t.CanceledOrders) == 0 {
			t.CanceledOrders = make(map[string]*Order)

		} else {
			if _, ok := t.CanceledOrders[id]; ok {
				return errors.New("Order found both in confirmed and cancel map")
			}
		}

		t.CanceledOrders[id] = order
		delete(t.ConfirmedOrders, id)
	} else {
		return errors.New("Can't cancel order. Not found in confirmed orders")
	}
	return nil
}

//executeOrder by given id and qty. If order qty was large than current position open qty then position will get state
//ClosedTrade and pointer to new opened position will be returned. All position values will be updated
func (t *Trade) executeOrder(id string, qty int, datetime time.Time) (*Trade, error) {

	order, ok := t.ConfirmedOrders[id]
	if !ok {
		return nil, errors.New("Can't execute order. Id not found in ConfirmedOrders")
	}

	if math.IsNaN(order.ExecPrice) || order.ExecPrice == 0 {
		panic("Panic: tried to execute order with zero or NaN execution price")
	}

	qtyLeft := order.Qty - order.ExecQty
	if qtyLeft < qty {
		return nil, errors.New("Can't execute order. Qty is greater than unexecuted order qty")
	}

	if qty == qtyLeft {
		if len(t.FilledOrders) == 0 {
			t.FilledOrders = make(map[string]*Order)
		} else {
			if _, ok := t.FilledOrders[id]; ok {
				return nil, errors.New("Can't execute order. ID already found in FilledOrders")
			}
		}
		order.State = FilledOrder
		t.FilledOrders[id] = order
		delete(t.ConfirmedOrders, id)
	} else {
		order.State = PartialFilledOrder
	}

	order.ExecQty += qty

	//Position update logic starts here
	switch t.Type {
	case FlatTrade:
		t.Qty = qty
		t.Id = order.Id
		if order.Side == OrderBuy {
			t.Type = LongTrade
		} else {
			if order.Side != OrderSell {
				panic("Unknow side for execution!")
			}
			t.Type = ShortTrade
		}
		t.OpenPrice = order.Price
		t.OpenValue = order.Price * float64(t.Qty)
		t.MarketValue = t.OpenValue
		t.OpenTime = datetime
		return nil, nil
	case ShortTrade:
		if order.Side == OrderSell {
			//Add to open short
			t.Qty += qty
			t.OpenValue += float64(qty) * order.ExecPrice
			t.OpenPrice = t.OpenValue / float64(t.Qty)
			t.MarketValue = float64(t.Qty) * order.ExecPrice
			t.OpenPnL = -(t.MarketValue - t.OpenValue)
			return nil, nil
		} else {
			//Cover open short position
			if qty < t.Qty {
				//Partial cover
				t.Qty -= qty
				t.ClosedPnL += -(order.ExecPrice - t.OpenPrice) * float64(qty)
				t.OpenValue = t.OpenPrice * float64(t.Qty)
				t.MarketValue = float64(t.Qty) * order.ExecPrice
				t.OpenPnL = -(t.MarketValue - t.OpenValue)
				return nil, nil
			} else {
				if qty == t.Qty {
					//Complete cover and return new FLAT position
					t.Qty -= qty
					t.ClosedPnL += -(order.ExecPrice - t.OpenPrice) * float64(qty)
					t.OpenValue = 0
					t.MarketValue = 0
					t.OpenPnL = 0
					t.Type = ClosedTrade
					t.CloseTime = datetime

					newTrade := newEmptyTrade(t.Symbol)
					newTrade.NewOrders = t.NewOrders
					newTrade.ConfirmedOrders = t.ConfirmedOrders

					t.NewOrders = make(map[string]*Order)
					t.ConfirmedOrders = make(map[string]*Order)

					return newTrade, nil

				} else {
					//Complete cover and open new LONG position
					newQty := qty - t.Qty
					t.ClosedPnL += -(order.ExecPrice - t.OpenPrice) * float64(t.Qty)
					t.Qty = 0
					t.OpenValue = 0
					t.MarketValue = 0
					t.OpenPnL = 0
					t.Type = ClosedTrade
					t.CloseTime = datetime

					newTrade := Trade{Symbol: t.Symbol, Qty: newQty, Id: order.Id, OpenTime: datetime, Type: LongTrade}
					newTrade.OpenPrice = order.ExecPrice
					newTrade.OpenValue = newTrade.OpenPrice * float64(newTrade.Qty)
					newTrade.MarketValue = newTrade.OpenValue

					newTrade.NewOrders = t.NewOrders
					newTrade.ConfirmedOrders = t.ConfirmedOrders
					newTrade.FilledOrders = map[string]*Order{id: order}
					newTrade.updateAllOrdersIDMap()

					t.NewOrders = make(map[string]*Order)
					t.ConfirmedOrders = make(map[string]*Order)

					return &newTrade, nil

				}
			}
		}
	case LongTrade:
		if order.Side == OrderBuy {
			//Add to open LONG
			t.Qty += qty
			t.OpenValue += float64(qty) * order.ExecPrice
			t.OpenPrice = t.OpenValue / float64(t.Qty)
			t.MarketValue = float64(t.Qty) * order.ExecPrice
			t.OpenPnL = t.MarketValue - t.OpenValue
			return nil, nil
		} else {
			if qty < t.Qty {
				//Partial cover LONG
				t.Qty -= qty
				t.ClosedPnL += (order.ExecPrice - t.OpenPrice) * float64(qty)
				t.OpenValue = t.OpenPrice * float64(t.Qty)
				t.MarketValue = float64(t.Qty) * order.ExecPrice
				t.OpenPnL = t.MarketValue - t.OpenValue
				return nil, nil
			} else {
				if qty == t.Qty {
					//Complete cover LONG and return new FLAT position
					t.Qty -= qty
					t.ClosedPnL += (order.ExecPrice - t.OpenPrice) * float64(qty)
					t.OpenValue = 0
					t.MarketValue = 0
					t.OpenPnL = 0
					t.Type = ClosedTrade
					t.CloseTime = datetime

					newTrade := newEmptyTrade(t.Symbol)
					newTrade.NewOrders = t.NewOrders
					newTrade.ConfirmedOrders = t.ConfirmedOrders

					t.NewOrders = make(map[string]*Order)
					t.ConfirmedOrders = make(map[string]*Order)

					return newTrade, nil

				} else {
					//Complete cover LONG and open new SHORT position
					newQty := qty - t.Qty
					t.ClosedPnL += (order.ExecPrice - t.OpenPrice) * float64(t.Qty)
					t.Qty = 0
					t.OpenValue = 0
					t.MarketValue = 0
					t.OpenPnL = 0
					t.Type = ClosedTrade
					t.CloseTime = datetime

					newTrade := Trade{Symbol: t.Symbol, Qty: newQty, Id: order.Id, OpenTime: datetime, Type: ShortTrade}
					newTrade.OpenPrice = order.ExecPrice
					newTrade.OpenValue = newTrade.OpenPrice * float64(newTrade.Qty)
					newTrade.MarketValue = newTrade.OpenValue
					newTrade.OpenPnL = 0

					newTrade.NewOrders = t.NewOrders
					newTrade.ConfirmedOrders = t.ConfirmedOrders
					newTrade.FilledOrders = map[string]*Order{id: order}
					newTrade.updateAllOrdersIDMap()

					t.NewOrders = make(map[string]*Order)
					t.ConfirmedOrders = make(map[string]*Order)

					return &newTrade, nil
				}
			}

		}

	}
	return nil, nil
}

func (t *Trade) updateAllOrdersIDMap() {
	t.AllOrdersIDMap = make(map[string]struct{})
	for _, o := range t.NewOrders {
		t.AllOrdersIDMap[o.Id] = struct{}{}
	}

	for _, o := range t.ConfirmedOrders {
		t.AllOrdersIDMap[o.Id] = struct{}{}
	}

	for _, o := range t.FilledOrders {
		t.AllOrdersIDMap[o.Id] = struct{}{}
	}

	for _, o := range t.RejectedOrders {
		t.AllOrdersIDMap[o.Id] = struct{}{}
	}

	for _, o := range t.CanceledOrders {
		t.AllOrdersIDMap[o.Id] = struct{}{}
	}

}

func (t *Trade) IsOpen() bool {
	if t.Qty != 0 {
		return true
	} else {
		return false
	}
}
