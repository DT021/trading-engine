package engine

import (
	"time"
	"alex/marketdata"
)

type event interface {
	getTime() time.Time
	getName() string


}

type CandleOpenEvent struct{
	Symbol string
	Time time.Time
	Candle *marketdata.Candle
}

func (c *CandleOpenEvent) getName() string{
	return "CandleOpenEvent"
}

func (c *CandleOpenEvent) getTime() time.Time{
	return  c.Time
}



type CandleCloseEvent struct{
	Symbol string
	Time time.Time
	Candle *marketdata.Candle
}

func (c *CandleCloseEvent) getName() string{
	return "CandleCloseEvent"
}

func (c *CandleCloseEvent) getTime() time.Time{
	return  c.Time
}


type NewTickEvent struct{
	Time time.Time
	Tick *marketdata.Tick
}

func (c *NewTickEvent) getName() string{
	return "NewTickEvent"
}

func (c *NewTickEvent) getTime() time.Time{
	return  c.Time
}


type NewOrderEvent struct{
	Time time.Time
}

func (c *NewOrderEvent) getName() string{
	return "NewOrderEvent"
}

func (c *NewOrderEvent) getTime() time.Time{
	return  c.Time
}


type OrderConfirmationEvent struct{
	Time time.Time
}

func (c *OrderConfirmationEvent) getName() string{
	return "OrderConfirmationEvent"
}

func (c *OrderConfirmationEvent) getTime() time.Time{
	return  c.Time
}


type OrderFillEvent struct{
	Symbol string
	Time time.Time
	ordId string
}

func (c *OrderFillEvent) getName() string{
	return "OrderConfirmationEvent"
}

func (c *OrderFillEvent) getTime() time.Time{
	return  c.Time
}


type OrderCancelEvent struct{
	Time time.Time
}

func (c *OrderCancelEvent) getName() string{
	return "OrderCancelEvent"
}

func (c *OrderCancelEvent) getTime() time.Time{
	return  c.Time
}



