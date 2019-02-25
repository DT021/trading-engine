package engine

import (
	"alex/marketdata"
	"fmt"
	"time"
)

type event interface {
	getTime() time.Time
	getName() string
}

type BaseEvent struct {
	Time   time.Time
	Symbol string
}

func (c *BaseEvent) getTime() time.Time {
	return c.Time
}

func be(datetime time.Time, symbol string) BaseEvent {
	b := BaseEvent{Time: datetime, Symbol: symbol}
	return b
}

type CandleOpenEvent struct {
	BaseEvent
	Price      float64
	CandleTime time.Time
}

func (c *CandleOpenEvent) getName() string {
	return "CandleOpenEvent"
}

type CandleCloseEvent struct {
	BaseEvent
	Candle *marketdata.Candle
}

func (c *CandleCloseEvent) getName() string {
	return "CandleCloseEvent"
}

type CandleHistoryRequestEvent struct {
	BaseEvent
	Candle *marketdata.Candle
}

func (c *CandleHistoryRequestEvent) getName() string {
	return "CandleHistoryRequestEvent"
}

type CandleHistoryEvent struct {
	BaseEvent
	Candles marketdata.CandleArray
}

func (c *CandleHistoryEvent) getName() string {
	return "CandleHistoryEvent"
}

type NewTickEvent struct {
	BaseEvent
	Tick *marketdata.Tick
}

func (c *NewTickEvent) getName() string {
	return "NewTickEvent"
}

type TickHistoryRequestEvent struct {
	BaseEvent
	Candle *marketdata.Candle
}

func (c *TickHistoryRequestEvent) getName() string {
	return "TickHistoryRequestEvent"
}

type TickHistoryEvent struct {
	BaseEvent
	Ticks marketdata.TickArray
}

func (c *TickHistoryEvent) getName() string {
	return "TickHistoryEvent"
}

type NewOrderEvent struct {
	BaseEvent
	LinkedOrder *Order
}

func (c *NewOrderEvent) getName() string {
	return "NewOrderEvent"
}

type OrderConfirmationEvent struct {
	BaseEvent
	OrdId string
}

func (c *OrderConfirmationEvent) getName() string {
	return "OrderConfirmationEvent"
}

type OrderFillEvent struct {
	BaseEvent
	OrdId string
	Price float64
	Qty   int
}

func (c *OrderFillEvent) getName() string {
	return "OrderFillEvent"
}

type OrderCancelEvent struct {
	BaseEvent
	OrdId string
}

func (c *OrderCancelEvent) getName() string {
	return "OrderCancelEvent"
}

type OrderCancelRequestEvent struct {
	BaseEvent
	OrdId string
}

func (c *OrderCancelRequestEvent) getName() string {
	return "OrderCancelRequestEvent"
}

type OrderReplaceRequestEvent struct {
	BaseEvent
	OrdId    string
	NewPrice float64
}

func (c *OrderReplaceRequestEvent) getName() string {
	return "OrderReplaceRequestEvent"
}

type OrderReplacedEvent struct {
	BaseEvent
	OrdId    string
	NewPrice float64
}

func (c *OrderReplacedEvent) getName() string {
	return "OrderReplacedEvent"
}

type OrderRejectedEvent struct {
	BaseEvent
	OrdId  string
	Reason string
}

func (c *OrderRejectedEvent) getName() string {
	return fmt.Sprintf("OrderRejectedEvent: %v Reason: %v", c.OrdId, c.Reason)
}

func (c *OrderRejectedEvent) getTime() time.Time {
	return c.Time
}

type BrokerPositionUpdateEvent struct {
	BaseEvent
}

func (c *BrokerPositionUpdateEvent) getName() string {
	return "BrokerPositionUpdateEvent"
}

type TimerTickEvent struct {
	BaseEvent
}

func (c *TimerTickEvent) getName() string {
	return "TimerTickEvent"
}

type EndOfDataEvent struct {
	BaseEvent
}

func (c *EndOfDataEvent) getName() string {
	return "EndOfDataEvent"
}

type BrokerNotifyEvent struct {
	BaseEvent
	InitialEvent event
}

func (c *BrokerNotifyEvent) getName() string {
	return "BrokerNotifyEvent"
}

type PortfolioNewPositionEvent struct {
	BaseEvent
	trade *Trade
}

func (c *PortfolioNewPositionEvent) getName() string {
	return "PortfolioNewPositionEvent"
}
