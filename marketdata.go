package engine

type IMarketData interface {
	Done() bool
	Next() *event
	Pop() *event
	Run()
}
