package engine

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTimerEventProducer_Run(t *testing.T) {
	events := make(chan event)
	errors := make(chan error)

	timer := TimerEventProducer{}
	timer.SetDelay(100)
	timer.SetFraction(100)
	timer.Connect(errors, events)

	n := 0

	timer.Run()
TIMER_LOOP:
	for {
		select {
		case e := <-events:
			assert.IsType(t, &TimerTickEvent{}, e)
			t.Log(e.getTime())
			n += 1
			if n > 10 {
				timer.Stop()
				break TIMER_LOOP
			}
		case e := <-errors:
			t.Error(e)
			timer.Stop()

		}
	}

	t.Log("Timer is done")
}
