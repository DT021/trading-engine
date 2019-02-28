package engine

import (
	"alex/marketdata"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	orderidLayout = "2006-01-02|15:04:05.000"
)

type CoreStrategyChannels struct {
	errors         chan error
	marketdata     chan event
	eventLogging   chan string
	signals        chan event
	broker         chan event
	portfolio      chan *PortfolioNewPositionEvent
	notifyBroker   chan *BrokerNotifyEvent
	brokerNotifier chan struct{}
	strategyDone   chan *StrategyFinishedEvent
}

func (c *CoreStrategyChannels) isValid() bool {
	if c.errors == nil {
		return false
	}

	if c.signals == nil {
		return false
	}
	if c.broker == nil {
		return false
	}
	if c.portfolio == nil {
		return false
	}

	if c.notifyBroker == nil {
		return false
	}

	if c.brokerNotifier == nil {
		return false
	}

	if c.marketdata == nil {
		return false
	}

	return true
}

type IStrategy interface {
	onCandleOpenHandler(e *CandleOpenEvent)
	onCandleCloseHandler(e *CandleCloseEvent)
	onTickHandler(e *NewTickEvent)
	onTickHistoryHandler(e *TickHistoryEvent)
	onCandleHistoryHandler(e *CandleHistoryEvent)

	ticks() marketdata.TickArray
	candles() marketdata.CandleArray
	Run()
	Finish()

	Connect(ch CoreStrategyChannels)
	setPortfolio(p *portfolioHandler)
	enableEventLogging()
	sendMarketData(e event)
}

type IUserStrategy interface {
	OnTick(b *BasicStrategy, tick *marketdata.Tick)
}

type BasicStrategy struct {
	portfolio        *portfolioHandler
	connected        bool
	Symbol           string
	Name             string
	NPeriods         int
	lastSeenTickTime time.Time
	tickCalls        int32

	ch CoreStrategyChannels

	terminationChan chan struct{}

	waitingConfirmation map[string]struct{}
	waitingN            int32

	closedTrades []*Trade
	currentTrade *Trade

	Ticks              marketdata.TickArray
	Candles            marketdata.CandleArray
	lastCandleOpen     float64
	lastCandleOpenTime time.Time

	strategy              IUserStrategy
	mostRecentTime        time.Time
	mut                   *sync.Mutex
	isEventLoggingEnabled bool
	log                   log.Logger
}

func (b *BasicStrategy) enableEventLogging() {
	b.isEventLoggingEnabled = true
}

func (b *BasicStrategy) sendMarketData(e event) {
	b.ch.marketdata <- e
}

func (b *BasicStrategy) readyForMarkerData() bool {
	if atomic.LoadInt32(&b.tickCalls) == 0 {
		return true
	}
	return false
}

func (b *BasicStrategy) init() {
	if b.currentTrade == nil {
		b.currentTrade = newFlatTrade(b.Symbol)
	}
	if len(b.closedTrades) == 0 {
		b.closedTrades = []*Trade{}
	}
}

func (b *BasicStrategy) Connect(ch CoreStrategyChannels) {
	if !ch.isValid() {
		panic("Core chans are not valid. Some of them is nil")
	}

	b.ch = ch
	b.terminationChan = make(chan struct{})
	b.waitingConfirmation = make(map[string]struct{})
	b.mut = &sync.Mutex{}
	b.connected = true
	b.tickCalls = 0
	f, err := os.OpenFile(b.Symbol+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	b.log.SetOutput(f)

}

func (b *BasicStrategy) setPortfolio(p *portfolioHandler) {
	b.portfolio = p
}

func (b *BasicStrategy) GetTotalPnL() float64 {
	return b.portfolio.totalPnL()
}

func (b *BasicStrategy) Finish() {
	/*go func() {
		b.terminationChan <- struct{}{}
	}()*/
}

//Strategy API calls
func (b *BasicStrategy) OnCandleClose() {

}

func (b *BasicStrategy) proxyEvent(e event) {
	switch i := e.(type) {
	case *OrderCancelEvent:
		b.onOrderCancelHandler(i)
	case *OrderCancelRejectEvent:
		b.onOrderCancelRejectHandler(i)
	case *OrderConfirmationEvent:
		b.onOrderConfirmHandler(i)
	case *OrderReplacedEvent:
		b.onOrderReplacedHandler(i)
	case *OrderReplaceRejectEvent:
		b.onOrderReplaceRejectHandler(i)
	case *OrderRejectedEvent:
		b.onOrderRejectedHandler(i)
	case *OrderFillEvent:
		b.onOrderFillHandler(i)
	case *StrategyRequestNotDeliveredEvent:
		b.onStrategyRequestNotDeliveredEventHandler(i)
	case *NewTickEvent:
		b.onTickHandler(i)
	case *TickHistoryEvent:
		b.onTickHistoryHandler(i)
	case *EndOfDataEvent:
		b.onEndOfDataHandler(i)
	default:
		panic("Unexpected event time in strategy: " + e.getName())
	}
}

func (b *BasicStrategy) onEndOfDataHandler(e *EndOfDataEvent) {
	b.terminationChan <- struct{}{}
}

func (b *BasicStrategy) Run() {
	go b.listenMarketData()
Loop:
	for {
		select {
		case e := <-b.ch.broker:
			b.sendEventForLogging(e)
			b.proxyEvent(e)
		case <-b.terminationChan:
			fmt.Println("Terminated")
			b.ch.strategyDone <- &StrategyFinishedEvent{strategy: b.Symbol}
			break Loop
		}

	}

}

func (b *BasicStrategy) listenMarketData() {

	var prevTime time.Time
Loop:
	for {
		select {
		case e := <-b.ch.marketdata:
			//b.sendEventForLogging(e)
			if e.getTime().Before(prevTime) {
				panic("Wrong order")
			}
			prevTime = e.getTime()
			b.proxyEvent(e)

			switch e.(type) {
			case *EndOfDataEvent:
				break Loop
			}
		default:
			continue Loop
		}
	}
}

func (b *BasicStrategy) sendEventForLogging(e event) {
	if !b.isEventLoggingEnabled {
		return
	}
	/*go func() {
		message := fmt.Sprintf("[SE:%v]  %+v", b.Symbol, e.String())
		b.ch.eventLogging <- message
	}()*/
	message := fmt.Sprintf("[SE:%v]  %+v", b.Symbol, e.String())
	b.log.Print(message)
}

func (b *BasicStrategy) OnCandleOpen() {

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

func (b *BasicStrategy) OrderIsConfirmed(ordId string) bool {
	return b.currentTrade.hasConfirmedOrderWithId(ordId)
}

func (b *BasicStrategy) NewLimitOrder(price float64, side OrderSide, qty int) (string, error) {
	order := Order{
		Side:   side,
		Qty:    qty,
		Symbol: b.Symbol,
		Price:  price,
		State:  NewOrder,
		Type:   LimitOrder,
		Time:   b.mostRecentTime,
		Id:     fmt.Sprintf("%v_%v_%v", price, LimitOrder, rand.Float64()),
	}

	err := b.newOrder(&order)
	return order.Id, err

}

func (b *BasicStrategy) newOrder(order *Order) error {
	if order.Symbol != b.Symbol {
		return errors.New("Can't put new order. Strategy symbol and order symbol are different. ")
	}
	if order.Id == "" {
		order.Id = order.Time.Format(orderidLayout)
	}

	if !order.isValid() {
		return errors.New("Order is not valid. ")
	}
	order.Id = b.Symbol + "|" + string(order.Side) + "|" + order.Id

	err := b.currentTrade.putNewOrder(order)

	if err != nil {
		go b.newError(err)
		return err
	}
	ordEvent := NewOrderEvent{
		LinkedOrder: order,
		BaseEvent:   be(order.Time, order.Symbol),
	}

	reqID := "$NO$" + order.Id
	if _, ok := b.waitingConfirmation[reqID]; ok {
		return errors.New("Order is already waiting for conf. ")
	} else {
		b.waitingConfirmation[reqID] = struct{}{}
		atomic.AddInt32(&b.waitingN, 1)
	}
	b.newSignal(&ordEvent)
	return nil
}

func (b *BasicStrategy) CancelOrder(ordID string) error {
	fmt.Println("Cancel order")
	if ordID == "" {
		err := ErrOrderIdIncorrect{
			OrdId:   ordID,
			Message: "Id is empty. ",
			Caller:  "CancelOrder func",
		}
		return &err
	}

	if !b.currentTrade.hasConfirmedOrderWithId(ordID) {
		err := ErrOrderNotFoundInConfirmedMap{
			ErrOrderNotFoundInOrdersMap{
				OrdId:   ordID,
				Message: "Id is empty. ",
				Caller:  "CancelOrder func",
			},
		}
		return &err
	}

	cancelReq := OrderCancelRequestEvent{
		OrdId:     ordID,
		BaseEvent: be(b.mostRecentTime, b.currentTrade.ConfirmedOrders[ordID].Symbol),
	}

	reqID := "$CAN$" + ordID
	if _, ok := b.waitingConfirmation[reqID]; ok {
		return errors.New("Request is already waiting for conf. ")
	} else {
		b.waitingConfirmation[reqID] = struct{}{}
		atomic.AddInt32(&b.waitingN, 1)
	}

	b.newSignal(&cancelReq)

	return nil
}

func (b *BasicStrategy) ReplaceOrder(ordID string, newPrice float64) error {
	if ordID == "" {
		err := ErrOrderIdIncorrect{
			OrdId:   ordID,
			Message: "Id is empty. ",
			Caller:  "ReplaceOrder func",
		}
		return &err
	}

	if !b.currentTrade.hasConfirmedOrderWithId(ordID) {
		err := ErrOrderNotFoundInConfirmedMap{
			ErrOrderNotFoundInOrdersMap{
				OrdId:   ordID,
				Message: "Id is empty. ",
				Caller:  "ReplaceOrder func",
			},
		}
		return &err
	}

	replaceReq := OrderReplaceRequestEvent{
		OrdId:     ordID,
		NewPrice:  newPrice,
		BaseEvent: be(b.mostRecentTime, b.Symbol),
	}

	reqID := "$REP$" + ordID
	if _, ok := b.waitingConfirmation[reqID]; ok {
		return errors.New("Request is already waiting for conf. ")
	} else {
		b.waitingConfirmation[reqID] = struct{}{}
		atomic.AddInt32(&b.waitingN, 1)
	}

	b.newSignal(&replaceReq)

	return nil
}

func (b *BasicStrategy) LastCandleOpen() float64 {
	return b.lastCandleOpen
}

func (b *BasicStrategy) tickIsValid(t *marketdata.Tick) bool {
	if t == nil {
		return false
	}
	if !t.HasTrade() && !t.HasQuote() {
		return false
	}
	return true
}

func (b *BasicStrategy) CandleIsValid(c *marketdata.Candle) bool {
	return true
}

//Market data events

func (b *BasicStrategy) onCandleCloseHandler(e *CandleCloseEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	if e == nil {
		return

	}
	if !b.CandleIsValid(e.Candle) || e.Candle == nil {
		return
	}

	b.putNewCandle(e.Candle)

	if b.currentTrade.IsOpen() {
		err := b.currentTrade.updatePnL(e.Candle.Close, e.Candle.Datetime)
		if err != nil {
			go b.newError(err)
		}
	}
	if len(b.Candles) < b.NPeriods {

		return
	}

	b.OnCandleClose()

}

func (b *BasicStrategy) onCandleOpenHandler(e *CandleOpenEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	if e == nil {
		return
	}

	if !e.CandleTime.Before(b.lastCandleOpenTime) {
		b.lastCandleOpen = e.Price
		b.lastCandleOpenTime = e.CandleTime
	}
	if b.currentTrade.IsOpen() {
		err := b.currentTrade.updatePnL(e.Price, e.CandleTime)
		if err != nil {
			go b.newError(err)
		}
	}

	b.OnCandleOpen()

}

//onCandleHistoryHandler puts historical candles in current array of candles.
func (b *BasicStrategy) onCandleHistoryHandler(e *CandleHistoryEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	if e.Candles == nil {
		return
	}
	if len(e.Candles) == 0 {
		return
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

	return
}

func (b *BasicStrategy) onTickHandler(e *NewTickEvent) {
	atomic.AddInt32(&b.tickCalls, 1)
	defer atomic.AddInt32(&b.tickCalls, -1)

	if e == nil {
		return
	}
	if !b.tickIsValid(e.Tick) {
		return
	}

	for atomic.LoadInt32(&b.waitingN) > 0 {

	}
	b.ch.signals <- e
	<-b.ch.brokerNotifier

	b.mut.Lock()
	defer b.mut.Unlock()

	b.mostRecentTime = e.Tick.Datetime

	b.putNewTick(e.Tick)
	if b.currentTrade.IsOpen() {
		err := b.currentTrade.updatePnL(e.Tick.LastPrice, e.Tick.Datetime)
		if err != nil {
			go b.newError(err)
		}
	}
	if len(b.Ticks) < b.NPeriods {
		return
	}

	b.strategy.OnTick(b, e.Tick)

}

//onTickHistoryHandler puts history ticks in current array of ticks. It doesn't produce any events.
func (b *BasicStrategy) onTickHistoryHandler(e *TickHistoryEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	if e.Ticks == nil {
		return
	}

	if len(e.Ticks) == 0 {
		return
	}

	allTicks := append(b.Ticks, e.Ticks...)

	var checkedTicks marketdata.TickArray

	for _, v := range allTicks {
		if v == nil {
			continue
		}
		if !b.tickIsValid(v) {
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

	return
}

//Order events

//onOrderFillHandler updates current state of order and current position
func (b *BasicStrategy) onOrderFillHandler(e *OrderFillEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	if e.Symbol != b.Symbol {
		go b.newError(errors.New("Mismatch symbols in fill event and position. "))
	}

	if e.Qty <= 0 {
		go b.newError(errors.New("Execution Qty is zero or less. "))
	}

	if math.IsNaN(e.Price) || e.Price <= 0 {
		go b.newError(errors.New("Price is NaN or less or equal to zero. "))
	}

	prevState := b.currentTrade.Type
	newPos, err := b.currentTrade.executeOrder(e.OrdId, e.Qty, e.Price, e.Time)

	if err != nil {
		go b.newError(err)
		return
	}
	if newPos != nil {
		if b.currentTrade.Type != ClosedTrade {
			go b.newError(errors.New("New position opened, but previous is not closed. "))
			return
		}
		b.closedTrades = append(b.closedTrades, b.currentTrade)
		b.currentTrade = newPos
		fmt.Println("New trade to portf event")
		b.notifyPortfolioAboutPosition(&PortfolioNewPositionEvent{be(e.getTime(), b.Symbol), b.currentTrade})

	} else {
		if prevState == FlatTrade {
			fmt.Println("New trade to portf event")
			b.notifyPortfolioAboutPosition(&PortfolioNewPositionEvent{be(e.getTime(), b.Symbol), b.currentTrade})
		}
	}

	b.ch.notifyBroker <- &BrokerNotifyEvent{be(e.Time, e.Symbol), e}

}

func (b *BasicStrategy) onOrderCancelHandler(e *OrderCancelEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	atomic.AddInt32(&b.waitingN, -1)
	delete(b.waitingConfirmation, "&CAN&"+e.OrdId)

	err := b.currentTrade.cancelOrder(e.OrdId)

	if err != nil {
		go b.newError(err)
		return
	}

}

func (b *BasicStrategy) onStrategyRequestNotDeliveredEventHandler(e *StrategyRequestNotDeliveredEvent) {
	if e.Request == nil {
		go b.newError(errors.New("onStrategyRequestNotDeliveredEventHandler got event with nil Request field"))
		return
	}

	panic("Not implemented") // Todo
}

func (b *BasicStrategy) onOrderCancelRejectHandler(e *OrderCancelRejectEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	atomic.AddInt32(&b.waitingN, -1)
	delete(b.waitingConfirmation, "&CAN&"+e.OrdId)

}

func (b *BasicStrategy) onOrderReplaceRejectHandler(e *OrderReplaceRejectEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	atomic.AddInt32(&b.waitingN, -1)
	delete(b.waitingConfirmation, "&REP&"+e.OrdId)

}

func (b *BasicStrategy) onOrderConfirmHandler(e *OrderConfirmationEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	atomic.AddInt32(&b.waitingN, -1)
	delete(b.waitingConfirmation, "&NO&"+e.OrdId)

	err := b.currentTrade.confirmOrder(e.OrdId)

	if err != nil {
		go b.newError(err)
		return
	}
}

func (b *BasicStrategy) onOrderReplacedHandler(e *OrderReplacedEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	atomic.AddInt32(&b.waitingN, -1)
	delete(b.waitingConfirmation, "&REP&"+e.OrdId)

	err := b.currentTrade.replaceOrder(e.OrdId, e.NewPrice)

	if err != nil {
		go b.newError(err)
	}

}

func (b *BasicStrategy) onOrderRejectedHandler(e *OrderRejectedEvent) {
	b.mut.Lock()
	defer b.mut.Unlock()

	atomic.AddInt32(&b.waitingN, -1)
	delete(b.waitingConfirmation, "&NO&"+e.OrdId)

	err := b.currentTrade.rejectOrder(e.OrdId, e.Reason)

	if err != nil {
		go b.newError(err)
		return
	}
}

//Timer events

func (b *BasicStrategy) onTimerTickHandler(e *TimerTickEvent) {

}

//Private funcs to work with data

func (b *BasicStrategy) newError(err error) {
	b.ch.errors <- err
}

func (b *BasicStrategy) newSignal(e event) {
	go func() {
		b.sendEventForLogging(e)
		b.ch.signals <- e
	}()

}

func (b *BasicStrategy) notifyPortfolioAboutPosition(e *PortfolioNewPositionEvent) {
	go func() {
		b.ch.portfolio <- e
	}()
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
