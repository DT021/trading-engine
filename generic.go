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
	OrderBuy  OrderSide = "B"
	OrderSell OrderSide = "S"

	NewOrder           OrderState = "NewOrder"
	ConfirmedOrder     OrderState = "ConfirmedOrder"
	FilledOrder        OrderState = "FilledOrder"
	PartialFilledOrder OrderState = "PartialFilledOrder"
	CanceledOrder      OrderState = "CanceledOrder"
	RejectedOrder      OrderState = "RejectedOrder"

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
	Mark1     string
	Mark2     string
}

//isValid returns if order has right prices (NaN for market orders and specified for Limit and Stop)
//valid order side, type, id and qty
func (o *Order) isValid() bool {
	if o.Symbol == "" {
		return false
	}

	if o.Id == "" {
		return false
	}

	if o.Qty <= 0 {
		return false
	}

	if o.State != NewOrder && o.State != ConfirmedOrder && o.State != FilledOrder && o.State != PartialFilledOrder && o.State != CanceledOrder {
		return false
	}

	if o.Side != OrderBuy && o.Side != OrderSell {
		return false
	}

	if o.Type == LimitOrder || o.Type == LimitOnClose || o.Type == LimitOnOpen || o.Type == StopOrder {
		if math.IsNaN(o.Price) || o.Price == 0 {
			return false
		}
	} else {
		if o.Type == MarketOrder || o.Type == MarketOnClose || o.Type == MarketOnOpen {
			if !math.IsNaN(o.Price) {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

func (o *Order) addExecution(price float64, qty int) error {
	if o.State == FilledOrder {
		return errors.New("Can't update order. Order is already filled")
	}

	if math.IsNaN(price) {
		return errors.New("Can't update order. Execution price is NaN")
	}
	if price <= 0 {
		return errors.New("Can't update order. Price less or equal zero")
	}
	if qty <= 0 {
		return errors.New("Can't update order. Execution qty is lezz or equal to zero")
	}
	if qty+o.ExecQty > o.Qty {
		return errors.New("Can't update order. Execution quantity is greater than lvs qty")
	}
	avgExecPrice := price
	if o.ExecQty > 0 {
		avgExecPrice = (float64(o.ExecQty)*o.ExecPrice + price*float64(qty)) / float64(o.ExecQty+qty)
	}

	o.ExecQty += qty
	o.ExecPrice = avgExecPrice
	if o.ExecQty == o.Qty {
		o.State = FilledOrder
	} else {
		o.State = PartialFilledOrder
	}
	return nil
}

func (o *Order) reject(reason string) error {
	if o.State != NewOrder {
		return errors.New("Can't reject order. It's not in status NewOrder")
	}
	if o.ExecQty > 0 {
		return errors.New("Can't reject order. Already has executed qty. Status should be PartialFilled")
	}

	o.State = RejectedOrder
	o.Mark1 = reason
	return nil
}

func (o *Order) cancel() error {
	if o.State == FilledOrder {
		return errors.New("Can't cancel filled order")
	}
	if o.State == NewOrder {
		return errors.New("Can't cancel new order.Should be confirmed")
	}

	o.State = CanceledOrder
	return nil
}

func (o *Order) confirm() error {
	if o.State != NewOrder {
		return errors.New("Can't confirm order. State is not NewOrder")
	}
	o.State = ConfirmedOrder
	return nil

}

func (o *Order) Created() bool {
	if o.Type != "" && o.Price != 0 {
		return true
	}
	return false
}

type TradeReturn struct {
	OpenPnL   float64
	ClosedPnL float64
	Time      time.Time
}

func newEmptyTrade(symbol string) *Trade {
	trade := Trade{Symbol: symbol, Qty: 0, Type: FlatTrade, OpenPrice: math.NaN(),
		ClosedPnL: 0, OpenPnL: 0, FilledOrders: make(map[string]*Order), CanceledOrders: make(map[string]*Order),
		NewOrders: make(map[string]*Order), ConfirmedOrders: make(map[string]*Order), RejectedOrders: make(map[string]*Order),
		AllOrdersIDMap: make(map[string]struct{})}

	return &trade
}

type Trade struct {
	Symbol      string
	Qty         int
	Type        TradeType
	FirstPrice  float64
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
	Returns         []*TradeReturn
	ClosedPnL       float64
	OpenPnL         float64
	Id              string
}

func (t *Trade) IsOpen() bool {
	if t.Type == LongTrade || t.Type == ShortTrade {
		if t.Qty == 0 {
			panic("Zero qty in open position")
		}
		return true
	}

	return false
}

//putNewOrder inserts order in NewOrders map. If there are order with same id in all orders
//map it will return error. There can't few orders even in different states with the same id
func (t *Trade) putNewOrder(o *Order) error {
	if t.Type == ClosedTrade {
		return errors.New("Can't put order to closed trade")
	}
	if !o.isValid() {
		return errors.New("Trying to put invalid order")
	}
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

		err := order.confirm()
		if err != nil {
			return err
		}
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
		err := order.cancel()
		if err != nil {
			return err
		}
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
func (t *Trade) executeOrder(id string, qty int, execPrice float64, datetime time.Time) (*Trade, error) {

	order, ok := t.ConfirmedOrders[id]
	if !ok {
		return nil, errors.New("Can't execute order. Id not found in ConfirmedOrders")
	}

	if math.IsNaN(execPrice) || execPrice == 0 {
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

		t.FilledOrders[id] = order
		delete(t.ConfirmedOrders, id)
	}

	err := order.addExecution(execPrice, qty)
	if err != nil {
		return nil, err
	}

	//Position update logic starts here
	switch t.Type {
	case FlatTrade:
		t.Qty = qty
		t.Id = order.Id
		t.FirstPrice = execPrice
		if order.Side == OrderBuy {
			t.Type = LongTrade
		} else {
			if order.Side != OrderSell {
				panic("Unknow side for execution!")
			}
			t.Type = ShortTrade
		}
		t.OpenPrice = execPrice
		t.OpenValue = execPrice * float64(t.Qty)
		t.MarketValue = t.OpenValue
		t.OpenTime = datetime
		return nil, nil
	case ShortTrade:
		if order.Side == OrderSell {
			//Add to open short
			t.Qty += qty
			t.OpenValue += float64(qty) * execPrice
			t.OpenPrice = t.OpenValue / float64(t.Qty)
			t.MarketValue = float64(t.Qty) * execPrice
			t.OpenPnL = -(t.MarketValue - t.OpenValue)
			return nil, nil
		} else {
			//Cover open short position
			if qty < t.Qty {
				//Partial cover
				t.Qty -= qty
				t.ClosedPnL += -(execPrice - t.OpenPrice) * float64(qty)
				t.OpenValue = t.OpenPrice * float64(t.Qty)
				t.MarketValue = float64(t.Qty) * execPrice
				t.OpenPnL = -(t.MarketValue - t.OpenValue)
				return nil, nil
			} else {
				if qty == t.Qty {
					//Complete cover and return new FLAT position
					t.Qty -= qty
					t.ClosedPnL += -(execPrice - t.OpenPrice) * float64(qty)
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
					t.ClosedPnL += -(execPrice - t.OpenPrice) * float64(t.Qty)
					t.Qty = 0
					t.OpenValue = 0
					t.MarketValue = 0
					t.OpenPnL = 0
					t.Type = ClosedTrade
					t.CloseTime = datetime

					newTrade := Trade{Symbol: t.Symbol, Qty: newQty, Id: order.Id, OpenTime: datetime, Type: LongTrade}
					newTrade.OpenPrice = execPrice
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
			t.OpenValue += float64(qty) * execPrice
			t.OpenPrice = t.OpenValue / float64(t.Qty)
			t.MarketValue = float64(t.Qty) * execPrice
			t.OpenPnL = t.MarketValue - t.OpenValue
			return nil, nil
		} else {
			if qty < t.Qty {
				//Partial cover LONG
				t.Qty -= qty
				t.ClosedPnL += (execPrice - t.OpenPrice) * float64(qty)
				t.OpenValue = t.OpenPrice * float64(t.Qty)
				t.MarketValue = float64(t.Qty) * execPrice
				t.OpenPnL = t.MarketValue - t.OpenValue
				return nil, nil
			} else {
				if qty == t.Qty {
					//Complete cover LONG and return new FLAT position
					t.Qty -= qty
					t.ClosedPnL += (execPrice - t.OpenPrice) * float64(qty)
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
					t.ClosedPnL += (execPrice - t.OpenPrice) * float64(t.Qty)
					t.Qty = 0
					t.OpenValue = 0
					t.MarketValue = 0
					t.OpenPnL = 0
					t.Type = ClosedTrade
					t.CloseTime = datetime

					newTrade := Trade{Symbol: t.Symbol, Qty: newQty, Id: order.Id, OpenTime: datetime, Type: ShortTrade}
					newTrade.OpenPrice = execPrice
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

//rejectOrder by given ID with given reject reason. Find order in NewOrdes map, change status and move it
//to RejectedOrders map
func (t *Trade) rejectOrder(id string, reason string) error {
	if order, ok := t.NewOrders[id]; ok {
		err := order.reject(reason)
		if err != nil {
			return err
		}
		if len(t.RejectedOrders) == 0 {
			t.RejectedOrders = make(map[string]*Order)

		} else {
			if _, ok := t.RejectedOrders[id]; ok {
				return errors.New("Order found both in confirmed and rejected map")
			}
		}

		t.RejectedOrders[id] = order
		delete(t.NewOrders, id)
	} else {
		return errors.New("Can't reject order. Not found in confirmed orders")
	}
	return nil
}

//updateAllOrdersIDMap updates AllOrdersIDMap from orders in other maps
//It can be used when we transfer orders from one position to another one
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


//updatePnL updates open pnl and positions market value for Long and Short positions
func (t *Trade) updatePnL(marketPrice float64, lastTime time.Time) error {
	t.MarketValue = marketPrice * float64(t.Qty)
	if t.Type == LongTrade {
		t.OpenPnL = t.MarketValue - t.OpenValue
	} else {
		if t.Type != ShortTrade {
			return errors.New("Can't update pnl for not open position")
		}
		t.OpenPnL = -(t.MarketValue - t.OpenValue)
	}

	t.Returns = append(t.Returns, &TradeReturn{t.OpenPnL, t.ClosedPnL, lastTime})
	return nil
}
