package engine

import "time"

type ITimerEventProducer interface {
	Run()
	Stop()
	Connect(errChan chan error, eventChan chan event)
	SetDelay(d int64)
	SetFraction(f int64)
}

type TimerEventProducer struct {
	delay     int64
	fraction  int64
	eventChan chan event
	errChan   chan error
	stopChan  chan struct{}
}

func (t *TimerEventProducer) Connect(errChan chan error, eventChan chan event) {
	if errChan == nil {
		panic("Error chan is nil")
	}

	if eventChan == nil {
		panic("Error chan is nil")
	}

	t.eventChan = eventChan
	t.errChan = errChan
	t.stopChan = make(chan struct{})
}

func (t *TimerEventProducer) Run() {
	tickChan := time.NewTicker(time.Duration(t.delay/t.fraction) * time.Nanosecond)


	go func() {
	TIMER_LOOP:
		for {
			select {
			case e := <-tickChan.C:
				te := TimerTickEvent{be(e, &Instrument{})}
				go t.newEvent(&te)
			case <-t.stopChan:
				tickChan.Stop()
				break TIMER_LOOP

			}
		}
	}()

}

func (t *TimerEventProducer) newEvent(e event) {
	t.eventChan <- e
}

func (t *TimerEventProducer) Stop() {
	go func() {
		t.stopChan <- struct{}{}
	}()

}

func (t *TimerEventProducer) SetDelay(d int64) {
	t.delay = d*1000000

}

func (t *TimerEventProducer) SetFraction(f int64) {
	t.fraction = f
}
