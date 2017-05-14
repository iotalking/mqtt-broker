package utils

import (
	"testing"
)

func TestEvent(t *testing.T) {
	ev := NewEvent()
	ev.Signal()
	ev.Signal()
	ev.Wait()
}
