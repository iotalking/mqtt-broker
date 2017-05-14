package utils

type Event struct {
	ch chan bool
}

func NewEvent() *Event {
	ev := &Event{
		ch: make(chan bool, 1),
	}

	return ev
}
func (this *Event) Signal() {
	select {
	case this.ch <- true:
	default:
	}
}

func (this *Event) Wait() {
	<-this.ch
}
