package engine

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"strconv"
	"time"
	"math"
)

func TestTrade_OrdersFlow(t *testing.T) {
	trade := Trade{Symbol: "Test"}
	t.Log("Put new order")
	{
		o := Order{Symbol: "Test", Id: "1", State: NewOrder}
		trade.putNewOrder(&o)

		assert.Equal(t, 1, len(trade.NewOrders))
	}

	t.Log("Put orders with wrong state and symbol")
	{
		o := Order{Symbol: "Test", Id: "2", State: FilledOrder}
		trade.putNewOrder(&o)

		assert.Equal(t, 1, len(trade.NewOrders))

		o = Order{Symbol: "Test2", Id: "55", State: NewOrder}
		trade.putNewOrder(&o)

		assert.Equal(t, 1, len(trade.NewOrders))
	}

	t.Log("Put order with that already in NewOrders map")
	{
		o := Order{Symbol: "Test", Id: "1", State: NewOrder}
		err := trade.putNewOrder(&o)

		assert.Equal(t, 1, len(trade.NewOrders))
		assert.Error(t, err, "Trying to add order in NewOrders with the ID that already in map")
	}

	t.Log("Put more orders")
	{

		i := 0
		for {
			i ++
			if i > 5 {
				break
			}
			o := Order{Symbol: "Test", Id: strconv.Itoa(i), State: NewOrder}
			trade.putNewOrder(&o)

		}

		assert.Equal(t, 5, len(trade.NewOrders))

	}

	t.Log("Confirm orders one by one")
	{
		i := 1
		for {
			trade.confirmOrder(strconv.Itoa(i))
			assert.Equal(t, 5-i, len(trade.NewOrders))
			assert.Equal(t, i, len(trade.ConfirmedOrders))
			i ++
			if i > 5 {
				break
			}
		}

		assert.Equal(t, 0, len(trade.NewOrders))
		assert.Equal(t, 5, len(trade.ConfirmedOrders))
	}

	t.Log("Add order with ID that already was in NewOrders")
	{
		o := Order{Symbol: "Test", Id: "1", State: NewOrder}
		trade.putNewOrder(&o)

		assert.Equal(t, 0, len(trade.NewOrders))
	}

	t.Log("Add order, confirm it and cancel")
	{
		o := Order{Symbol: "Test", Id: "10", State: NewOrder}
		trade.putNewOrder(&o)

		assert.Equal(t, 1, len(trade.NewOrders))
		assert.Equal(t, 5, len(trade.ConfirmedOrders))

		err := trade.cancelOrder("10")
		assert.Error(t, err, "Can't cancel order. Not found in confirmed orders")

		trade.confirmOrder("10")
		assert.Equal(t, 6, len(trade.ConfirmedOrders))
		err = trade.cancelOrder("10")
		assert.Equal(t, 1, len(trade.CanceledOrders))
		assert.Equal(t, 5, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

	}
}

func newValidOrder(price float64, side OrderSide, qty int, id string) *Order {
	o := Order{Symbol: "Test", Qty: qty, Side: side, Id: id, Price: price}
	return &o
}

func TestTrade_OrdersExecution(t *testing.T) {
	trade := newEmptyTrade("Test", "TestID")

	t.Log("Add few valid orders - long and short")
	{
		trade.putNewOrder(newValidOrder(20, OrderBuy, 100, "1"))
		trade.putNewOrder(newValidOrder(15, OrderBuy, 200, "2"))
		trade.putNewOrder(newValidOrder(25, OrderSell, 100, "3"))

		trade.confirmOrder("1")
		trade.confirmOrder("2")
		trade.confirmOrder("3")

		assert.Equal(t, 3, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))
		assert.Equal(t, 0, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.CanceledOrders))

	}

	t.Log("Execute long orders")
	{
		execTime0 := time.Now()
		trade.executeOrder("1", 100, execTime0)
		assert.Equal(t, 2, len(trade.ConfirmedOrders))
		assert.Equal(t, 1, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, 100, trade.Qty)
		assert.Equal(t, 20, trade.OpenPrice)
		assert.Equal(t, 20*100, trade.OpenValue)
		assert.Equal(t, 20*100, trade.MarketValue)
		assert.Equal(t, execTime0, trade.OpenTime)
		assert.Equal(t, 0, trade.OpenPnL)

		execTime := time.Now().Add(20 * time.Minute)
		trade.executeOrder("2", 100, execTime)
		assert.Equal(t, 2, len(trade.ConfirmedOrders))
		assert.Equal(t, 1, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, 200, trade.Qty)
		assert.Equal(t, 17.5, trade.OpenPrice)
		assert.Equal(t, 17.5*200, trade.OpenValue)
		assert.Equal(t, 17.5*200, trade.MarketValue)
		assert.Equal(t, execTime0, trade.OpenTime)
		assert.Equal(t, -500, trade.OpenPnL)

		execTime = time.Now().Add(22 * time.Minute)
		trade.executeOrder("2", 100, execTime)
		assert.Equal(t, 1, len(trade.ConfirmedOrders))
		assert.Equal(t, 2, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, 300, trade.Qty)
		assert.Equal(t, 16.66, math.Floor(trade.OpenPrice))
		assert.Equal(t, execTime0, trade.OpenTime)
		assert.Equal(t, -500, trade.OpenPnL)

	}

	t.Log("Execute short orders")
	{
		execTime := time.Now().Add(20 * time.Minute)
		trade.executeOrder("3", 100, execTime)
		assert.Equal(t, 0, len(trade.ConfirmedOrders))
		assert.Equal(t, 3, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, 200, trade.Qty)
		assert.Equal(t, 16.66, math.Floor(trade.OpenPrice))
		openPnl := (25.0-trade.OpenPrice)*200
		assert.Equal(t, openPnl, trade.OpenPnL)
		closedPnL := (25-trade.OpenPrice)*100
		assert.Equal(t, closedPnL, trade.ClosedPnL)
		assert.Equal(t, trade.OpenPrice*200, trade.OpenValue)
		assert.Equal(t, 25*200, trade.MarketValue)


		trade.putNewOrder(newValidOrder(25, OrderSell, 400, "4"))
		trade.confirmOrder("4")

		assert.Equal(t, 1, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		trade.putNewOrder(newValidOrder(20, OrderBuy, 100, "5"))
		trade.confirmOrder("5")

		trade.putNewOrder(newValidOrder(10, OrderBuy, 200, "6"))
		trade.confirmOrder("6")

		trade.putNewOrder(newValidOrder(40, OrderSell, 100, "7"))
		trade.confirmOrder("7")

		execTime = time.Now().Add(20 * time.Minute)

		newTrade, err:= trade.executeOrder("4", 400, execTime)

		if err!=nil{
			t.Error(err)
			t.Fail()
		}

		assert.NotNil(t, newTrade)

		assert.Equal(t, ClosedTrade, trade.Type)
		assert.Equal(t, execTime, trade.CloseTime)
		assert.Equal(t, 0, trade.Qty)
		assert.Equal(t, 0, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))
		assert.Equal(t, 4, len(trade.FilledOrders))
		assert.Equal(t, 0, trade.OpenValue)
		assert.Equal(t, 0, trade.MarketValue)

		assert.Equal(t, 200, newTrade.Qty)
		assert.Equal(t, ShortTrade, newTrade.Type)
		assert.Equal(t, 3, len(newTrade.ConfirmedOrders))
		assert.Equal(t, 1, len(newTrade.FilledOrders))
		assert.Equal(t, 0, len(newTrade.NewOrders))
		assert.Equal(t, 25, newTrade.OpenPrice)
		assert.Equal(t, execTime, newTrade.OpenTime)
		assert.Equal(t, 4, len(newTrade.AllOrdersIDMap))
		assert.Equal(t, 0, newTrade.ClosedPnL)
		assert.Equal(t, 0, newTrade.OpenPnL)
		assert.Equal(t, 25*200, newTrade.OpenValue)
		assert.Equal(t, 25*200, newTrade.MarketValue)






	}

}
