package engine

import "sync"

type eventsSliceStorage struct {
	events []event
	mut *sync.Mutex
}

func (w *eventsSliceStorage) add(e event){
	w.mut.Lock()
	w.events = append(w.events, e)
	w.mut.Unlock()
}

func (w *eventsSliceStorage) storedEvents() []event{
	return w.events
}
