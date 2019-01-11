package engine

type IStrategy interface {
	OnCandleOpen()
	OnCandleClose()
	OnFill()
	OnCancel()
	OnTick()
	OnOrderConfirm()
}
