package engine

import (
	"alex/marketdata"
	"sort"
	"time"
	"errors"
	"math"
)

const (
	orderidLayout = "2006-01-02|15:04:05.000"
)

type IStrategy interface {
	onCandleOpenHandler(e *CandleOpenEvent)
	onCandleCloseHandler(e *CandleCloseEvent)
	onTickHandler(e *NewTickEvent)
	onTickHistoryHandler(e *TickHistoryEvent) []*event
	onCandleHistoryHandler(e *CandleHistoryEvent) []*event

	onOrderFillHandler(e *OrderFillEvent)
	onOrderCancelHandler(e *OrderCancelEvent)
	onOrderConfirmHandler(e *OrderConfirmationEvent)
	onOrderReplacedHandler(e *OrderReplacedEvent) []*event
	onOrderRejectedHandler(e *OrderRejectedEvent)

	onBrokerPositionHandler(e *BrokerPositionUpdateEvent) []*event

	onTimerTickHandler(e *TimerTickEvent) []*event

	ticks() marketdata.TickArray
	candles() marketdata.CandleArray
}

type BasicStrategy struct {
	connected          bool
	Symbol             string
	Name               string
	NPeriods           int
	closedTrades       []*Trade
	currentTrade       *Trade
	Ticks              marketdata.TickArray
	Candles            marketdata.CandleArray
	lastCandleOpen     float64
	lastCandleOpenTime time.Time
	eventChan          chan *event
	errorsChan         chan error
}

func (b *BasicStrategy) init() {
	if b.currentTrade == nil {
		b.currentTrade = newFlatTrade(b.Symbol)
	}
	if len(b.closedTrades) == 0 {
		b.closedTrades = []*Trade{}
	}
}

func (b *BasicStrategy) connect(eventChan chan *event, errorsChan chan error) {
	if eventChan == nil {
		panic("Can't connect stategy. Event channel is nil")
	}

	if errorsChan == nil {
		panic("Can't connect strategy. Errors chan is nil")
	}

	b.eventChan = eventChan
	b.errorsChan = errorsChan
	b.connected = true
}

//Strategy API calls
func (b *BasicStrategy) OnCandleClose() {

}

func (b *BasicStrategy) OnCandleOpen() {

}

func (b *BasicStrategy) OnTick() {

}

func (b *BasicStrategy) OpenOrders() map[string]*Order {
	return b.currentTrade.ConfirmedOrders
}

func (b *BasicStrategy) Position() int {
	if b.currentTrade.Type == FlatTrade || b.currentTrade.Type == ClosedTrade {
		return 0
	}
	pos := b.currentTrade.Qty
	if b.currentTrade.Type == ShortTrade {
		pos = -pos
	}

	return pos

}

func (b *BasicStrategy) NewOrder(order *Order) error {
	if order.Symbol != b.Symbol {
		return errors.New("Can't put new order. Strategy symbol and order symbol are different")
	}
	if order.Id == "" {
		order.Id = time.Now().Format(orderidLayout)
	}

	if !order.isValid() {
		return errors.New("Order is not valid")
	}
	order.Id = b.Symbol + "|" + string(order.Side) + "|" + order.Id
	b.currentTrade.putNewOrder(order)
	ordEvent := NewOrderEvent{LinkedOrder: order, Time: order.Time}
	go b.newEvent(&ordEvent)
	return nil
}

func (b *BasicStrategy) CancelOrder(ordID string) error {
	if ordID == "" {
		return errors.New("Order Id not specified")
	}
	if !b.currentTrade.hasOrderWithID(ordID) {
		return errors.New("Order ID not found in confirmed orders")
	}
	cancelReq := OrderCancelRequestEvent{OrdId: ordID, Time: time.Now()}

	go b.newEvent(&cancelReq)

	return nil
}

func (b *BasicStrategy) LastCandleOpen() float64 {
	return b.lastCandleOpen
}

func (b *BasicStrategy) TickIsValid(t *marketdata.Tick) bool {
	return true
}

func (b *BasicStrategy) CandleIsValid(c *marketdata.Candle) bool {
	return true
}

//Market data events

func (b *BasicStrategy) onCandleCloseHandler(e *CandleCloseEvent) {
	if e == nil {
		return

	}
	if !b.CandleIsValid(e.Candle) || e.Candle == nil {
		return
	}

	b.putNewCandle(e.Candle)
	if b.currentTrade.IsOpen() {
		b.currentTrade.updatePnL(e.Candle.Close, e.Candle.Datetime)
	}
	if len(b.Candles) < b.NPeriods {
		return
	}
	b.OnCandleClose()

}

func (b *BasicStrategy) onCandleOpenHandler(e *CandleOpenEvent) {
	if e == nil {
		return
	}

	if !e.CandleTime.Before(b.lastCandleOpenTime) {
		b.lastCandleOpen = e.Price
		b.lastCandleOpenTime = e.CandleTime
	}
	if b.currentTrade.IsOpen() {
		b.currentTrade.updatePnL(e.Price, e.CandleTime)
	}

	b.OnCandleOpen()

}

//onCandleHistoryHandler puts historical candles in current array of candles.
func (b *BasicStrategy) onCandleHistoryHandler(e *CandleHistoryEvent) []*event {
	if e.Candles == nil {
		return nil
	}
	if len(e.Candles) == 0 {
		return nil
	}

	allCandles := append(b.Candles, e.Candles...)
	listedCandleTimes := make(map[time.Time]struct{})
	var checkedCandles marketdata.CandleArray

	for _, v := range allCandles {
		if v == nil {
			continue
		}
		if !b.CandleIsValid(v) {
			continue
		}
		if _, ok := listedCandleTimes[v.Datetime]; ok {
			continue
		}

		checkedCandles = append(checkedCandles, v)
		listedCandleTimes[v.Datetime] = struct{}{}
	}

	sort.SliceStable(checkedCandles, func(i, j int) bool {
		return checkedCandles[i].Datetime.Unix() < checkedCandles[j].Datetime.Unix()
	})

	if len(checkedCandles) > b.NPeriods {
		b.Candles = checkedCandles[len(checkedCandles)-b.NPeriods:]
	} else {
		b.Candles = checkedCandles
	}

	b.updateLastCandleOpen()

	return nil
}

func (b *BasicStrategy) onTickHandler(e *NewTickEvent) {
	if e == nil {
		return
	}
	if !b.TickIsValid(e.Tick) || e.Tick == nil {
		return
	}

	b.putNewTick(e.Tick)
	if b.currentTrade.IsOpen() {
		b.currentTrade.updatePnL(e.Tick.LastPrice, e.Tick.Datetime)
	}
	if len(b.Ticks) < b.NPeriods {
		return
	}
	b.OnTick()

}

//onTickHistoryHandler puts history ticks in current array of ticks. It doesn't produce any events.
func (b *BasicStrategy) onTickHistoryHandler(e *TickHistoryEvent) []*event {
	if e.Ticks == nil {
		return nil
	}

	if len(e.Ticks) == 0 {
		return nil
	}

	allTicks := append(b.Ticks, e.Ticks...)

	var checkedTicks marketdata.TickArray

	for _, v := range allTicks {
		if v == nil {
			continue
		}
		if !b.TickIsValid(v) {
			continue
		}

		checkedTicks = append(checkedTicks, v)

	}

	sort.SliceStable(checkedTicks, func(i, j int) bool {
		return checkedTicks[i].Datetime.Unix() < checkedTicks[j].Datetime.Unix()
	})

	if len(checkedTicks) > b.NPeriods {
		b.Ticks = checkedTicks[len(checkedTicks)-b.NPeriods:]
	} else {
		b.Ticks = checkedTicks
	}

	return nil
}

//Order events

//onOrderFillHandler updates current state of order and current position
func (b *BasicStrategy) onOrderFillHandler(e *OrderFillEvent) {

	if e.Symbol != b.Symbol {
		go b.error(errors.New("Mismatch symbols in fill event and position"))
	}

	if e.Qty <= 0 {
		go b.error(errors.New("Execution Qty is zero or less."))
	}

	if math.IsNaN(e.Price) || e.Price <= 0 {
		go b.error(errors.New("Price is NaN or less or equal to zero."))
	}
	newPos, err := b.currentTrade.executeOrder(e.OrdId, e.Qty, e.Price, e.Time)
	if err != nil {
		go b.error(err)
		return
	}
	if newPos != nil {
		if b.currentTrade.Type != ClosedTrade {
			go b.error(errors.New("New position opened, but previous is not closed"))
			return
		}
		b.closedTrades = append(b.closedTrades, b.currentTrade)
		b.currentTrade = newPos
	}

}

func (b *BasicStrategy) onOrderCancelHandler(e *OrderCancelEvent) {
	err := b.currentTrade.cancelOrder(e.OrdId)
	if err != nil {
		go b.error(err)
		return
	}

}

func (b *BasicStrategy) onOrderConfirmHandler(e *OrderConfirmationEvent) {
	err := b.currentTrade.confirmOrder(e.OrdId)
	if err != nil {
		go b.error(err)
		return
	}
}

func (b *BasicStrategy) onOrderReplacedHandler(e *OrderReplacedEvent) []*event {
	err := b.currentTrade.replaceOrder(e.OrdId, e.NewPrice)
	if err != nil {
		go b.error(err)
	}
	return nil
}

func (b *BasicStrategy) onOrderRejectedHandler(e *OrderRejectedEvent) {
	err := b.currentTrade.rejectOrder(e.OrdId, e.Reason)
	if err != nil {
		go b.error(err)
		return
	}
}

//Broker events

func (b *BasicStrategy) onBrokerPositionHandler(e *BrokerPositionUpdateEvent) []*event {
	return nil
}

//Timer events

func (b *BasicStrategy) onTimerTickHandler(e *TimerTickEvent) []*event {
	return nil
}

//Private funcs to work with data

func (b *BasicStrategy) error(err error) {
	if b.errorsChan != nil {
		b.errorsChan <- err
	}
}

func (b *BasicStrategy) newEvent(e event) {
	if b.eventChan != nil {
		b.eventChan <- &e

	}
}

func (b *BasicStrategy) putNewCandle(candle *marketdata.Candle) {
	if candle == nil {
		return
	}

	sortIt := false
	if len(b.Candles) > 0 && candle.Datetime.Before(b.Candles[len(b.Candles)-1].Datetime) {
		sortIt = true
	}

	if len(b.Candles) < b.NPeriods {
		b.Candles = append(b.Candles, candle)
		b.updateLastCandleOpen()
		return
	}
	b.Candles = append(b.Candles[1:], candle)

	if sortIt {
		sort.SliceStable(b.Candles, func(i, j int) bool {
			return b.Candles[i].Datetime.Unix() < b.Candles[j].Datetime.Unix()
		})
	}

	b.updateLastCandleOpen()
	return
}

func (b *BasicStrategy) putNewTick(tick *marketdata.Tick) {
	if tick == nil {
		return
	}
	sortIt := false
	if len(b.Ticks) > 0 && tick.Datetime.Before(b.Ticks[len(b.Ticks)-1].Datetime) {
		sortIt = true
	}

	if len(b.Ticks) < b.NPeriods {
		b.Ticks = append(b.Ticks, tick)
		return
	}
	b.Ticks = append(b.Ticks[1:], tick)

	if sortIt {
		sort.SliceStable(b.Ticks, func(i, j int) bool {
			return b.Ticks[i].Datetime.Unix() < b.Ticks[j].Datetime.Unix()
		})
	}
	return
}

func (b *BasicStrategy) updateLastCandleOpen() {
	if len(b.Candles) == 0 {
		return
	}
	lastCandleInHist := b.Candles[len(b.Candles)-1]
	if lastCandleInHist.Datetime.After(b.lastCandleOpenTime) {
		b.lastCandleOpen = lastCandleInHist.Open
		b.lastCandleOpenTime = lastCandleInHist.Datetime
	}

}

func (b *BasicStrategy) ticks() marketdata.TickArray {
	return b.Ticks
}

func (b *BasicStrategy) candles() marketdata.CandleArray {
	return b.Candles
}
