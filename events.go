package engine

import (
	"time"
	"alex/marketdata"
	"fmt"
)

type event interface {
	getTime() time.Time
	getName() string
}

type CandleOpenEvent struct {
	Symbol     string
	Time       time.Time
	Price      float64
	CandleTime time.Time
}

func (c *CandleOpenEvent) getName() string {
	return "CandleOpenEvent"
}

func (c *CandleOpenEvent) getTime() time.Time {
	return c.Time
}

type CandleCloseEvent struct {
	Symbol string
	Time   time.Time
	Candle *marketdata.Candle
}

func (c *CandleCloseEvent) getName() string {
	return "CandleCloseEvent"
}

func (c *CandleCloseEvent) getTime() time.Time {
	return c.Time
}

type CandleHistoryRequestEvent struct {
	Symbol string
	Time   time.Time
	Candle *marketdata.Candle
}

func (c *CandleHistoryRequestEvent) getName() string {
	return "CandleHistoryRequestEvent"
}

func (c *CandleHistoryRequestEvent) getTime() time.Time {
	return c.Time
}

type CandleHistoryEvent struct {
	Symbol  string
	Time    time.Time
	Candles marketdata.CandleArray
}

func (c *CandleHistoryEvent) getName() string {
	return "CandleHistoryEvent"
}

func (c *CandleHistoryEvent) getTime() time.Time {
	return c.Time
}

type NewTickEvent struct {
	Time time.Time
	Tick *marketdata.Tick
}

func (c *NewTickEvent) getName() string {
	return "NewTickEvent"
}

func (c *NewTickEvent) getTime() time.Time {
	return c.Time
}

type TickHistoryRequestEvent struct {
	Symbol string
	Time   time.Time
	Candle *marketdata.Candle
}

func (c *TickHistoryRequestEvent) getName() string {
	return "TickHistoryRequestEvent"
}

func (c *TickHistoryRequestEvent) getTime() time.Time {
	return c.Time
}

type TickHistoryEvent struct {
	Symbol string
	Time   time.Time
	Ticks  marketdata.TickArray
}

func (c *TickHistoryEvent) getName() string {
	return "TickHistoryEvent"
}

func (c *TickHistoryEvent) getTime() time.Time {
	return c.Time
}

type NewOrderEvent struct {
	LinkedOrder *Order
	Time        time.Time
}

func (c *NewOrderEvent) getName() string {
	return "NewOrderEvent"
}

func (c *NewOrderEvent) getTime() time.Time {
	return c.Time
}

type OrderConfirmationEvent struct {
	OrdId string
	Time  time.Time
}

func (c *OrderConfirmationEvent) getName() string {
	return "OrderConfirmationEvent"
}

func (c *OrderConfirmationEvent) getTime() time.Time {
	return c.Time
}

type OrderFillEvent struct {
	OrdId  string
	Symbol string
	Price  float64
	Qty    int
	Time   time.Time
}

func (c *OrderFillEvent) getName() string {
	return "OrderConfirmationEvent"
}

func (c *OrderFillEvent) getTime() time.Time {
	return c.Time
}

type OrderCancelEvent struct {
	OrdId string
	Time  time.Time
}

func (c *OrderCancelEvent) getName() string {
	return "OrderCancelEvent"
}

func (c *OrderCancelEvent) getTime() time.Time {
	return c.Time
}

type OrderCancelRequestEvent struct {
	OrdId string
	Time  time.Time
}

func (c *OrderCancelRequestEvent) getName() string {
	return "OrderCancelRequestEvent"
}

func (c *OrderCancelRequestEvent) getTime() time.Time {
	return c.Time
}

type OrderReplaceRequestEvent struct {
	Time time.Time
}

func (c *OrderReplaceRequestEvent) getName() string {
	return "OrderReplaceRequestEvent"
}

func (c *OrderReplaceRequestEvent) getTime() time.Time {
	return c.Time
}

type OrderReplacedEvent struct {
	OrdId    string
	NewPrice float64
	Time     time.Time
}

func (c *OrderReplacedEvent) getName() string {
	return "OrderReplacedEvent"
}

func (c *OrderReplacedEvent) getTime() time.Time {
	return c.Time
}

type OrderRejectedEvent struct {
	OrdId  string
	Reason string
	Time   time.Time
}

func (c *OrderRejectedEvent) getName() string {
	return fmt.Sprintf("OrderRejectedEvent: %v Reason: %v", c.OrdId, c.Reason)
}

func (c *OrderRejectedEvent) getTime() time.Time {
	return c.Time
}

type BrokerPositionUpdateEvent struct {
	Time time.Time
}

func (c *BrokerPositionUpdateEvent) getName() string {
	return "BrokerPositionUpdateEvent"
}

func (c *BrokerPositionUpdateEvent) getTime() time.Time {
	return c.Time
}

type TimerTickEvent struct {
	Time time.Time
}

func (c *TimerTickEvent) getName() string {
	return "TimerTickEvent"
}

func (c *TimerTickEvent) getTime() time.Time {
	return c.Time
}
