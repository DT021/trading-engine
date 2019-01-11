package engine

import "time"

type event interface {
	getTime() time.Time
	getName() string


}

type CandleOpenEvent struct{
	Time time.Time
}

func (c *CandleOpenEvent) getName() string{
	return "CandleOpenEvent"
}

func (c *CandleOpenEvent) getTime() time.Time{
	return  c.Time
}



type CandleCloseEvent struct{
	Time time.Time
}

func (c *CandleCloseEvent) getName() string{
	return "CandleCloseEvent"
}

func (c *CandleCloseEvent) getTime() time.Time{
	return  c.Time
}


type NewTickEvent struct{
	Time time.Time
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
	Time time.Time
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



