package session

import (
	"testing"
	"time"
)

func TestCloseSendingChan(t *testing.T) {
	c := make(chan byte, 1)
	defer func() {
		err := recover()
		if err == nil {
			t.Fail()
		}
	}()
	go func() {
		time.Sleep(2 * time.Second)
		close(c)
	}()
	c <- 1
	c <- 1
}
