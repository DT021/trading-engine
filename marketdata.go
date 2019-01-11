package engine

type IMarketData interface {
	Done() bool
	Next() *event
	Pop() *event
	Run()
}

type ITimerEventProducer interface { //Todo подумать над этим
	Next()
	Pop() *event

}
