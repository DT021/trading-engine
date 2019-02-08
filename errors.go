package engine

import (
	"fmt"
	"alex/marketdata"
)

type ErrBrokenTick struct {
	Tick   marketdata.Tick
	Message string
	Caller  string
}

func (e *ErrBrokenTick) Error() string {
	return fmt.Sprintf("%v: ErrBrokenTick (id:%v). %v", e.Caller, e.Tick, e.Message)

}

type ErrInvalidOrder struct {
	OrdId   string
	Message string
	Caller  string
}

func (e *ErrInvalidOrder) Error() string {
	return fmt.Sprintf("%v: ErrInvalidOrder (id:%v). %v", e.Caller, e.OrdId, e.Message)

}

type ErrUnknownOrderSide struct {
	OrdId   string
	Message string
	Caller  string
}

func (e *ErrUnknownOrderSide) Error() string {
	return fmt.Sprintf("%v: ErrUnknownOrderSide (id:%v). %v", e.Caller, e.OrdId, e.Message)

}

type ErrUnknownOrderType struct {
	OrdId   string
	Message string
	Caller  string
}

func (e *ErrUnknownOrderType) Error() string {
	return fmt.Sprintf("%v: ErrUnknownOrderType (id:%v). %v", e.Caller, e.OrdId, e.Message)

}

type ErrUnexpectedOrderType struct {
	OrdId        string
	ActualType   string
	ExpectedType string
	Message      string
	Caller       string
}

func (e *ErrUnexpectedOrderType) Error() string {
	return fmt.Sprintf("%v: ErrUnexpectedOrderType (id:%v). Expected: %v, Actual:%v. %v", e.Caller, e.OrdId, e.ExpectedType, e.ActualType, e.Message)

}


type ErrUnexpectedOrderState struct {
	OrdId         string
	ActualState   string
	ExpectedState string
	Message       string
	Caller        string
}

func (e *ErrUnexpectedOrderState) Error() string {
	return fmt.Sprintf("%v: ErrUnexpectedOrderState (id:%v). Expected: %v, Actual:%v. %v", e.Caller, e.OrdId, e.ExpectedState, e.ActualState, e.Message)

}


type ErrOrderNotFoundInConfirmedMap struct {
	OrdId   string
	Message string
	Caller  string
}

func (e *ErrOrderNotFoundInConfirmedMap) Error() string {
	return fmt.Sprintf("%v: ErrOrderNotFoundInConfirmedMap (id:%v). %v", e.Caller, e.OrdId, e.Message)

}



