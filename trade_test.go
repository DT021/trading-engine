package engine

import (
	"github.com/stretchr/testify/assert"
	"math"
	"strconv"
	"testing"
	"time"
)

func newTestOrderTime() time.Time {
	return time.Date(2010, 1, 5, 10, 0, 0, 0, time.UTC)
}

func newTestOpgOrderTime() time.Time {
	return time.Date(2010, 1, 5, 9, 00, 10, 15, time.UTC)
}

func newTestOrder(price float64, side OrderSide, qty int64, id string) *Order {
	o := Order{
		Time:        newTestOrderTime(),
		Ticker:      newTestInstrument(),
		Qty:         qty,
		Side:        side,
		Id:          id,
		Price:       price,
		Tif:         GTCTIF,
		Destination: "ARCA",
		ExecPrice:   math.NaN(),
		State:       NewOrder,
		Type:        LimitOrder,
	}
	return &o
}

func TestOrder_isValid(t *testing.T) {
	t.Log("Check test order creation method")
	{
		order := newTestOrder(10, OrderBuy, 100, "1")
		assert.True(t, order.isValid())
	}
	t.Log("Check some invalid orders")
	{
		order := newTestOrder(10, OrderBuy, 100, "1")
		order.Price = math.NaN()
		assert.False(t, order.isValid())

		order.Type = MarketOnOpen
		assert.True(t, order.isValid())

		order.Price = 10
		assert.False(t, order.isValid())

		order.Type = LimitOnOpen
		assert.True(t, order.isValid())

		order.Qty = 0
		assert.False(t, order.isValid())

		order.Qty = 500
		order.Id = ""
		assert.False(t, order.isValid())

		order.Id = "5"
		assert.True(t, order.isValid())

		order.State = ""
		assert.False(t, order.isValid())

		order.State = NewOrder
		assert.True(t, order.isValid())

		order.Side = ""
		assert.False(t, order.isValid())

		order.Side = OrderSell
		assert.True(t, order.isValid())

		order.Type = ""
		assert.False(t, order.isValid())

	}
}

func TestOrder_addExecution(t *testing.T) {
	t.Log("Simple addExecution to order")
	{
		order := newTestOrder(10, OrderSell, 200, "1")
		err := order.addExecution(11, 200)
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, FilledOrder, order.State)
		assert.Equal(t, 11.0, order.ExecPrice)
		assert.Equal(t, int64(200), order.ExecQty)

		t.Log("Try to add execution to filled order")
		{
			err = order.addExecution(11, 200)
			assert.Error(t, err, "Can't update order. Order is already filled")
		}

	}

	t.Log("add execution with wrong params")
	{
		order := newTestOrder(10, OrderSell, 200, "1")

		err := order.addExecution(math.NaN(), 200)
		assert.NotNil(t, err)
		assert.Equal(t, NewOrder, order.State)

		err = order.addExecution(1, -200)
		assert.NotNil(t, err)
		assert.Equal(t, NewOrder, order.State)

		err = order.addExecution(1, 300)
		assert.NotNil(t, err)
		assert.Equal(t, NewOrder, order.State)

		err = order.addExecution(1, 100)
		assert.Nil(t, err)
		assert.Equal(t, PartialFilledOrder, order.State)

		err = order.addExecution(1, 200)
		assert.NotNil(t, err)
		assert.Equal(t, PartialFilledOrder, order.State)

	}

	t.Log("Add few executions to one order")
	{
		order := newTestOrder(20, OrderSell, 400, "2")
		err := order.addExecution(20, 100)
		assert.Nil(t, err)
		assert.Equal(t, PartialFilledOrder, order.State)
		assert.Equal(t, 20.0, order.ExecPrice)

		err = order.addExecution(10, 100)
		assert.Nil(t, err)
		assert.Equal(t, PartialFilledOrder, order.State)
		assert.Equal(t, 15.0, order.ExecPrice)

		err = order.addExecution(25, 200)
		assert.Nil(t, err)
		assert.Equal(t, FilledOrder, order.State)
		assert.Equal(t, 20.0, order.ExecPrice)

		err = order.addExecution(25, 300)
		assert.NotNil(t, err)

	}

}

func TestTrade_OrdersFlow(t *testing.T) {
	trade := Trade{Ticker: newTestInstrument()}
	t.Log("Put new order")
	{

		o := newTestOrder(20, OrderBuy, 100, "#1")
		err := trade.putNewOrder(o)
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, 1, len(trade.NewOrders))
	}

	t.Log("Put orders with wrong state and symbol")
	{
		o := newTestOrder(20, OrderBuy, 100, "2")
		o.State = FilledOrder

		err := trade.putNewOrder(o)
		if err == nil {
			t.Fatal("Error expected")
		}

		assert.Equal(t, 1, len(trade.NewOrders))

		o = newTestOrder(20, OrderBuy, 100, "55")
		o.Ticker.Symbol = "Test2"
		err = trade.putNewOrder(o)
		if err == nil {
			t.Fatal("Error expected")
		}

		assert.Equal(t, 1, len(trade.NewOrders))
	}

	t.Log("Put order with that already in NewOrders map")
	{
		o := newTestOrder(20, OrderBuy, 100, "#1")
		err := trade.putNewOrder(o)

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

			o := newTestOrder(20, OrderBuy, 100, strconv.Itoa(i))
			err := trade.putNewOrder(o)
			if err != nil {
				t.Fatalf("%v for %v", err, i)
			}

		}

		assert.Equal(t, 6, len(trade.NewOrders))

	}

	t.Log("Confirm orders one by one")
	{
		i := 1
		for {
			err := trade.confirmOrder(strconv.Itoa(i))
			if err != nil {
				t.Fatalf("Found: %v, for %v", err, i)
			}
			assert.Equal(t, 6-i, len(trade.NewOrders))
			assert.Equal(t, i, len(trade.ConfirmedOrders))
			i ++
			if i > 5 {
				break
			}
		}
		err := trade.confirmOrder("#1")
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 0, len(trade.NewOrders))
		assert.Equal(t, 6, len(trade.ConfirmedOrders))
	}

	t.Log("Add order with ID that already was in NewOrders")
	{
		o := newTestOrder(20, OrderBuy, 100, "1")
		err := trade.putNewOrder(o)
		if err == nil {
			t.Fatal("Error expected")
		}

		assert.Equal(t, 0, len(trade.NewOrders))
	}

	t.Log("Add order, confirm it and cancel")
	{
		o := newTestOrder(20, OrderBuy, 100, "10")
		err := trade.putNewOrder(o)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 1, len(trade.NewOrders))
		assert.Equal(t, 6, len(trade.ConfirmedOrders))

		err = trade.cancelOrder("10")
		assert.Error(t, err, "Can't cancel order. Not found in confirmed orders")

		err = trade.confirmOrder("10")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 7, len(trade.ConfirmedOrders))
		err = trade.cancelOrder("10")
		assert.Equal(t, 1, len(trade.CanceledOrders))
		assert.Equal(t, 6, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

	}

	t.Log("Put new order and reject it")
	{
		err := trade.putNewOrder(newTestOrder(10, OrderSell, 500, "888"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.rejectOrder("888", "Not shortable")
		assert.Nil(t, err)
		assert.Len(t, trade.RejectedOrders, 1)
		assert.Len(t, trade.NewOrders, 0)
		assert.Equal(t, RejectedOrder, trade.RejectedOrders["888"].State)
		assert.Equal(t, "Not shortable", trade.RejectedOrders["888"].Mark1)
	}

	t.Log("Put order and replace it")
	{
		err := trade.putNewOrder(newTestOrder(200, OrderBuy, 500, "999"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("999")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 200.00, trade.ConfirmedOrders["999"].Price)

		err = trade.replaceOrder("999", 150.0)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 150.00, trade.ConfirmedOrders["999"].Price)

		mktOrder := newTestOrder(10, OrderBuy, 100, "*8")
		mktOrder.Price = math.NaN()
		mktOrder.Type = MarketOrder

		err = trade.putNewOrder(mktOrder)
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("*8")
		assert.Nil(t, err)

		assert.True(t, math.IsNaN(trade.ConfirmedOrders["*8"].Price))

		err = trade.replaceOrder("*8", 200)
		assert.NotNil(t, err)

		assert.True(t, math.IsNaN(trade.ConfirmedOrders["*8"].Price))

	}
}

func TestTrade_OrdersExecution(t *testing.T) {
	trade := newFlatTrade(newTestInstrument())

	t.Log("Add few valid orders - long and short")
	{
		err := trade.putNewOrder(newTestOrder(20, OrderBuy, 100, "1"))
		if err != nil {
			t.Error(err)
		}

		err = trade.putNewOrder(newTestOrder(15, OrderBuy, 200, "2"))
		if err != nil {
			t.Error(err)
		}

		err = trade.putNewOrder(newTestOrder(25, OrderSell, 100, "3"))
		if err != nil {
			t.Error(err)
		}

		err = trade.confirmOrder("1")
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("2")
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("3")
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 3, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))
		assert.Equal(t, 0, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.CanceledOrders))

		for _, o := range trade.ConfirmedOrders {
			assert.Equal(t, ConfirmedOrder, o.State)
		}

	}

	t.Log("Execute long orders")
	{
		//EXECUTE FIRST ORDER
		execTime0 := time.Now()
		_, err := trade.executeOrder("1", 100, 20, execTime0)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 2, len(trade.ConfirmedOrders))
		for _, o := range trade.ConfirmedOrders {
			assert.Equal(t, ConfirmedOrder, o.State)
		}

		assert.Equal(t, 1, len(trade.FilledOrders))
		for _, o := range trade.FilledOrders {
			assert.Equal(t, FilledOrder, o.State)
		}

		assert.Equal(t, 20.0, trade.FilledOrders["1"].ExecPrice)
		assert.Equal(t, int64(100), trade.FilledOrders["1"].ExecQty)

		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, int64(100), trade.Qty)
		assert.Equal(t, 20.0, trade.OpenPrice)
		assert.Equal(t, 20.0*100, trade.OpenValue)
		assert.Equal(t, 20.0*100, trade.MarketValue)
		assert.Equal(t, execTime0, trade.OpenTime)
		assert.Equal(t, 0.0, trade.OpenPnL)

		//EXECUTE NEXT ORDER
		execTime := time.Now().Add(20 * time.Minute)
		_, err = trade.executeOrder("2", 100, 15, execTime)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 15.0, trade.ConfirmedOrders["2"].ExecPrice)
		assert.Equal(t, int64(100), trade.ConfirmedOrders["2"].ExecQty)
		assert.Equal(t, PartialFilledOrder, trade.ConfirmedOrders["2"].State)

		assert.Equal(t, 2, len(trade.ConfirmedOrders))
		assert.Equal(t, 1, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, int64(200), trade.Qty)
		assert.Equal(t, 17.5, trade.OpenPrice)
		assert.Equal(t, 17.5*200, trade.OpenValue)
		assert.Equal(t, 15.0*200, trade.MarketValue)
		assert.Equal(t, execTime0, trade.OpenTime)
		assert.Equal(t, -500.0, trade.OpenPnL)

		//EXECUTE SAME ORDER - COMPLITE FILL
		execTime = time.Now().Add(22 * time.Minute)
		_, err = trade.executeOrder("2", 100, 15, execTime)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 1, len(trade.ConfirmedOrders))
		assert.Equal(t, 2, len(trade.FilledOrders))
		assert.Equal(t, 15.0, trade.FilledOrders["2"].ExecPrice)
		assert.Equal(t, int64(200), trade.FilledOrders["2"].ExecQty)
		assert.Equal(t, FilledOrder, trade.FilledOrders["2"].State)

		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, int64(300), trade.Qty)
		assert.Equal(t, 16.66, math.Floor(trade.OpenPrice*100)/100)
		assert.Equal(t, execTime0, trade.OpenTime)
		assert.Equal(t, -500.0, trade.OpenPnL)

	}

	t.Log("Execute short orders")
	{
		//EXECUTE FIRST ORDER TO COVER EXISTING POSITION
		execTime := time.Now().Add(20 * time.Minute)
		_, err := trade.executeOrder("3", 100, 25, execTime)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 25.0, trade.FilledOrders["3"].ExecPrice)
		assert.Equal(t, int64(100), trade.FilledOrders["3"].ExecQty)
		assert.Equal(t, FilledOrder, trade.FilledOrders["3"].State)

		assert.Equal(t, 0, len(trade.ConfirmedOrders))
		assert.Equal(t, 3, len(trade.FilledOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, int64(200), trade.Qty)
		assert.Equal(t, 16.66, math.Floor(trade.OpenPrice*100)/100)
		openPnl := (25.0 - trade.OpenPrice) * 200
		assert.Equal(t, openPnl, trade.OpenPnL)
		closedPnL := (25 - trade.OpenPrice) * 100
		assert.Equal(t, closedPnL, trade.ClosedPnL)
		assert.Equal(t, trade.OpenPrice*200, trade.OpenValue)
		assert.Equal(t, 25.0*200, trade.MarketValue)

		//PUT MORE SELL ORDERS

		err = trade.putNewOrder(newTestOrder(25, OrderSell, 400, "4"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("4")
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 1, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))

		err = trade.putNewOrder(newTestOrder(20, OrderBuy, 100, "5"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("5")
		if err != nil {
			t.Fatal(err)
		}

		err = trade.putNewOrder(newTestOrder(10, OrderBuy, 200, "6"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("6")
		if err != nil {
			t.Fatal(err)
		}

		err = trade.putNewOrder(newTestOrder(40, OrderSell, 100, "7"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("7")
		if err != nil {
			t.Fatal(err)
		}

		execTime = time.Now().Add(20 * time.Minute)

		//EXECUTE ONE ADDED SHORT ORDER: COVER POSITION AND OPEN NEW
		newTrade, err := trade.executeOrder("4", 400, 25, execTime)

		assert.Equal(t, 25.0, trade.FilledOrders["4"].ExecPrice)
		assert.Equal(t, int64(400), trade.FilledOrders["4"].ExecQty)
		assert.Equal(t, FilledOrder, trade.FilledOrders["4"].State)

		if err != nil {
			t.Error(err)
			t.Fail()
		}

		assert.NotNil(t, newTrade)

		assert.Equal(t, ClosedTrade, trade.Type)
		assert.Equal(t, execTime, trade.CloseTime)
		assert.Equal(t, int64(0), trade.Qty)
		assert.Equal(t, 0, len(trade.ConfirmedOrders))
		assert.Equal(t, 0, len(trade.NewOrders))
		assert.Equal(t, 4, len(trade.FilledOrders))
		assert.Equal(t, 0.0, trade.OpenValue)
		assert.Equal(t, 0.0, trade.MarketValue)

		assert.Equal(t, int64(200), newTrade.Qty)
		assert.Equal(t, ShortTrade, newTrade.Type)
		assert.Equal(t, 3, len(newTrade.ConfirmedOrders))
		assert.Equal(t, 0, len(newTrade.FilledOrders))
		assert.Equal(t, 0, len(newTrade.NewOrders))
		assert.Equal(t, 25.0, newTrade.OpenPrice)
		assert.Equal(t, execTime, newTrade.OpenTime)
		assert.Equal(t, 3, len(newTrade.AllOrdersIDMap))
		assert.Equal(t, 0.0, newTrade.ClosedPnL)
		assert.Equal(t, 0.0, newTrade.OpenPnL)
		assert.Equal(t, 25.0*200, newTrade.OpenValue)
		assert.Equal(t, 25.0*200, newTrade.MarketValue)

		//EXECUTE SHORT ORDER ADDED TO PREVIOUS POSITION
		p, err := newTrade.executeOrder("7", 100, 20, execTime)
		assert.Nil(t, p)
		assert.Equal(t, 1, len(newTrade.FilledOrders))

		assert.Equal(t, 20.0, newTrade.FilledOrders["7"].ExecPrice)
		assert.Equal(t, int64(100), newTrade.FilledOrders["7"].ExecQty)
		assert.Equal(t, FilledOrder, newTrade.FilledOrders["7"].State)

		assert.Equal(t, int64(300), newTrade.Qty)
		assert.Equal(t, ShortTrade, newTrade.Type)
		assert.Equal(t, 2, len(newTrade.ConfirmedOrders))

		assert.Equal(t, math.Floor((25.0*200+100*20.0)*100/300), math.Floor(newTrade.OpenPrice*100))
		assert.Equal(t, 0.0, newTrade.ClosedPnL)
		assert.Equal(t, 1000.0, newTrade.OpenPnL)
		assert.Equal(t, newTrade.OpenPrice*300, newTrade.OpenValue)
		assert.Equal(t, 20.0*300, newTrade.MarketValue)

		//ADD EXTRA BUY ORDER
		err = newTrade.putNewOrder(newTestOrder(10, OrderBuy, 200, "8"))
		if err != nil {
			t.Fatal(err)
		}
		err = newTrade.confirmOrder("8")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 3, len(newTrade.ConfirmedOrders))

		//CANCEL ONE ORDER FROM PREVIOUS POSITION
		err = newTrade.cancelOrder("6")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 2, len(newTrade.ConfirmedOrders))
		assert.Equal(t, 1, len(newTrade.CanceledOrders))
		assert.Equal(t, CanceledOrder, newTrade.CanceledOrders["6"].State)

		//PARTIAL EXECUTE RECENTLY ADDED ORDER
		execTime = time.Now()
		p, err = newTrade.executeOrder("8", 100, 15, execTime)
		assert.Nil(t, p)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(newTrade.ConfirmedOrders))
		assert.Equal(t, PartialFilledOrder, newTrade.ConfirmedOrders["8"].State)
		assert.Equal(t, 15.0, newTrade.ConfirmedOrders["8"].ExecPrice)
		assert.Equal(t, int64(100), newTrade.ConfirmedOrders["8"].ExecQty)

		assert.Equal(t, ShortTrade, newTrade.Type)
		assert.Equal(t, int64(200), newTrade.Qty)
		assert.Equal(t, math.Floor((25.0*200+100*20.0)*100/300), math.Floor(newTrade.OpenPrice*100))
		assert.Equal(t, newTrade.OpenPrice*200, newTrade.OpenValue)
		assert.Equal(t, 15.0*200, newTrade.MarketValue)

		expectedClosedPnL := ((25.0*200 + 100*20.0) * 100 / 300) - 15.0*100
		assert.Equal(t, math.Floor(expectedClosedPnL*100), math.Floor(newTrade.ClosedPnL*100))

		expectedOpenPnL := ((25.0*200 + 100*20.0) * 200 / 300) - 15.0*200
		assert.Equal(t, math.Floor(expectedOpenPnL*100), math.Floor(newTrade.OpenPnL*100))

		//TRY TO EXECUTE CANCELED ORDER
		p, err = newTrade.executeOrder("6", 100, 15, execTime)
		assert.Nil(t, p)
		assert.NotNil(t, err)

		assert.Equal(t, ShortTrade, newTrade.Type)
		assert.Equal(t, int64(200), newTrade.Qty)
		assert.Equal(t, math.Floor((25.0*200+100*20.0)*100/300), math.Floor(newTrade.OpenPrice*100))
		assert.Equal(t, newTrade.OpenPrice*200, newTrade.OpenValue)
		assert.Equal(t, 15.0*200, newTrade.MarketValue)

		expectedClosedPnL = ((25.0*200 + 100*20.0) * 100 / 300) - 15.0*100
		assert.Equal(t, math.Floor(expectedClosedPnL*100), math.Floor(newTrade.ClosedPnL*100))

		expectedOpenPnL = ((25.0*200 + 100*20.0) * 200 / 300) - 15.0*200
		assert.Equal(t, math.Floor(expectedOpenPnL*100), math.Floor(newTrade.OpenPnL*100))

		//EXECUTE BUY ORDER FROM PREVIOUS POSITION
		p, err = newTrade.executeOrder("5", 100, 20, execTime)
		assert.Nil(t, p)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(newTrade.ConfirmedOrders))
		assert.Equal(t, 2, len(newTrade.FilledOrders))

		assert.Equal(t, FilledOrder, newTrade.FilledOrders["5"].State)
		assert.Equal(t, 20.0, newTrade.FilledOrders["5"].ExecPrice)
		assert.Equal(t, int64(100), newTrade.FilledOrders["5"].ExecQty)

		assert.Equal(t, ShortTrade, newTrade.Type)
		assert.Equal(t, int64(100), newTrade.Qty)
		assert.Equal(t, math.Floor((25.0*200+100*20.0)*100/300), math.Floor(newTrade.OpenPrice*100))
		assert.Equal(t, newTrade.OpenPrice*100, newTrade.OpenValue)
		assert.Equal(t, 20.0*100, newTrade.MarketValue)

		expectedClosedPnL = ((25.0*200 + 100*20.0) * 200 / 300) - 15.0*100 - 20*100
		assert.Equal(t, math.Floor(expectedClosedPnL*100), math.Floor(newTrade.ClosedPnL*100))

		expectedOpenPnL = ((25.0*200 + 100*20.0) * 100 / 300) - 20.0*100
		assert.Equal(t, math.Floor(expectedOpenPnL*100), math.Floor(newTrade.OpenPnL*100))

		//EXECUTE RECENTLY PARTIALLY FILLED ORDER AND CLOSE POSITION
		execTimeEnd := time.Now().Add(time.Minute * 5)
		p, err = newTrade.executeOrder("8", 100, 25, execTimeEnd)
		assert.NotNil(t, p)
		assert.Nil(t, err)

		assert.Equal(t, ClosedTrade, newTrade.Type)
		assert.Equal(t, int64(0), newTrade.Qty)

		assert.Equal(t, 0, len(newTrade.ConfirmedOrders))
		assert.Equal(t, 3, len(newTrade.FilledOrders))

		assert.Equal(t, FilledOrder, newTrade.FilledOrders["8"].State)
		assert.Equal(t, 20.0, newTrade.FilledOrders["8"].ExecPrice)
		assert.Equal(t, int64(200), newTrade.FilledOrders["8"].ExecQty)

		assert.Equal(t, 0.0, newTrade.OpenPnL)
		assert.Equal(t, 0.0, newTrade.MarketValue)
		assert.Equal(t, 0.0, newTrade.OpenValue)
		assert.Equal(t, execTimeEnd, newTrade.CloseTime)

		expectedClosedPnL = 25.0*200 + 100*20.0 - 15.0*100 - 20*100 - 25.0*100
		assert.True(t, math.Abs(expectedClosedPnL-newTrade.ClosedPnL) < 0.0002)

		assert.Equal(t, FlatTrade, p.Type)
		assert.Equal(t, int64(0), p.Qty)
		assert.Equal(t, 0, len(p.NewOrders))
		assert.Equal(t, 0, len(p.ConfirmedOrders))
		assert.Equal(t, 0, len(p.FilledOrders))
		assert.Equal(t, 0, len(p.CanceledOrders))

		//TRY TO PUT ORDER TO CLOSED POSITION
		err = newTrade.putNewOrder(newTestOrder(10, OrderBuy, 100, "200"))
		assert.NotNil(t, err)
		assert.Equal(t, 0, len(newTrade.NewOrders))

	}

	t.Log("Check id, first price")
	{
		execTime := time.Now()
		trade = newFlatTrade(newTestInstrument())
		err := trade.putNewOrder(newTestOrder(10, OrderSell, 100, "22"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("22")
		if err != nil {
			t.Fatal(err)
		}
		_, err = trade.executeOrder("22", 100, 22, execTime)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, "22", trade.Id)
		assert.Equal(t, 22.0, trade.FirstPrice)

	}

}

func TestTrade_PartialFillAndReverse(t *testing.T) {
	// Test case when we have partial fill and get new position with partial filled order

	trade := newFlatTrade(newTestInstrument())
	t.Log("Add, confirm valid order and execute it. Trade should be LONG")
	{
		err := trade.putNewOrder(newTestOrder(20, OrderBuy, 200, "1"))
		if err != nil {
			t.Error(err)
		}

		err = trade.confirmOrder("1")
		if err != nil {
			t.Error(err)
		}

		newTrade, err := trade.executeOrder("1", 200, 20, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		assert.Nil(t, newTrade)
		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, int64(200), trade.Qty)

	}

	t.Log("Add, confirm and partial fill short order. Expect new SHORT trade with partial filled order")
	{
		err := trade.putNewOrder(newTestOrder(20, OrderSell, 400, "2"))
		if err != nil {
			t.Error(err)
		}

		err = trade.confirmOrder("2")
		if err != nil {
			t.Error(err)
		}

		newTrade, err := trade.executeOrder("2", 289, 21, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		assert.NotNil(t, newTrade)
		assert.Equal(t, ClosedTrade, trade.Type)
		assert.Equal(t, ShortTrade, newTrade.Type)
		assert.Equal(t, int64(89), newTrade.Qty)
		ord, ok := newTrade.ConfirmedOrders["2"]
		assert.True(t, ok)
		assert.Equal(t, PartialFilledOrder, ord.State)
		assert.Len(t, newTrade.FilledOrders, 0)
		assert.Len(t, newTrade.ConfirmedOrders, 1)

		t.Log("Complete fill previous order. Add to currently open short position")
		{
			newTrade2, err := newTrade.executeOrder("2", 111, 21, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			assert.Nil(t, newTrade2)
			assert.Equal(t, ShortTrade, newTrade.Type)
			assert.Equal(t, int64(200), newTrade.Qty)
			assert.Equal(t, FilledOrder, ord.State)
			_, ok := newTrade.ConfirmedOrders["2"]
			assert.False(t, ok)

			_, ok = newTrade.FilledOrders["2"]
			assert.True(t, ok)
		}
	}

}

func TestTrade_updatePnL(t *testing.T) {
	trade := newFlatTrade(newTestInstrument())

	t.Log("Add execution to a trade")
	{
		err := trade.putNewOrder(newTestOrder(10, OrderSell, 100, "1"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("1")
		if err != nil {
			t.Fatal(err)
		}
		_, err = trade.executeOrder("1", 100, 10, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		assert.Len(t, trade.FilledOrders, 1)
		assert.Equal(t, ShortTrade, trade.Type)

	}

	t.Log("Add updates")
	{
		err := trade.updatePnL(11, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, -100.0, trade.OpenPnL)
		assert.Equal(t, 0.0, trade.ClosedPnL)
		assert.Len(t, trade.Returns, 1)
		assert.Equal(t, -100.0, trade.Returns[0].OpenPnL)

		err = trade.updatePnL(9, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 100.0, trade.OpenPnL)
		assert.Equal(t, 0.0, trade.ClosedPnL)
		assert.Len(t, trade.Returns, 2)
		assert.Equal(t, -100.0, trade.Returns[0].OpenPnL)
		assert.Equal(t, 100.0, trade.Returns[1].OpenPnL)

		err = trade.updatePnL(10, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 0.0, trade.OpenPnL)
		assert.Equal(t, 0.0, trade.ClosedPnL)
		assert.Len(t, trade.Returns, 3)
		assert.Equal(t, -100.0, trade.Returns[0].OpenPnL)
		assert.Equal(t, 100.0, trade.Returns[1].OpenPnL)
		assert.Equal(t, 0.0, trade.Returns[2].OpenPnL)

	}

	t.Log("Close trade and add updates to it")
	{
		err := trade.putNewOrder(newTestOrder(5, OrderBuy, 100, "2"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("2")
		if err != nil {
			t.Fatal(err)
		}
		_, err = trade.executeOrder("2", 100, 5, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		assert.Len(t, trade.FilledOrders, 2)
		assert.Equal(t, ClosedTrade, trade.Type)

		assert.Equal(t, 500.0, trade.ClosedPnL)
		assert.Equal(t, 0.0, trade.OpenPnL)
		assert.Len(t, trade.Returns, 3)

		err = trade.updatePnL(10, time.Now())

		assert.NotNil(t, err)

		assert.Equal(t, 500.0, trade.ClosedPnL)
		assert.Equal(t, 0.0, trade.OpenPnL)
		assert.Len(t, trade.Returns, 3)

	}

	t.Log("Create long trade and check updates")
	{
		trade = newFlatTrade(newTestInstrument())
		err := trade.putNewOrder(newTestOrder(10, OrderBuy, 100, "1"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("1")
		if err != nil {
			t.Fatal(err)
		}
		_, err = trade.executeOrder("1", 100, 11, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, LongTrade, trade.Type)
		assert.Equal(t, 0.0, trade.OpenPnL)
		assert.Equal(t, 0.0, trade.ClosedPnL)
		assert.Equal(t, 11.0, trade.OpenPrice)

		err = trade.updatePnL(12, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 100.0, trade.OpenPnL)
		assert.Equal(t, 0.0, trade.ClosedPnL)

		assert.Len(t, trade.Returns, 1)

		err = trade.updatePnL(15, time.Now())

		assert.Equal(t, 400.0, trade.OpenPnL)
		assert.Equal(t, 0.0, trade.ClosedPnL)

		assert.Equal(t, 400.0, trade.Returns[1].OpenPnL)

		err = trade.putNewOrder(newTestOrder(18, OrderSell, 100, "2"))
		if err != nil {
			t.Fatal(err)
		}
		err = trade.confirmOrder("2")
		if err != nil {
			t.Fatal(err)
		}
		_, err = trade.executeOrder("2", 100, 18, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, 0.0, trade.OpenPnL)
		assert.Equal(t, 700.0, trade.ClosedPnL)

	}
}
