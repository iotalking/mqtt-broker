package dashboard

import (
	"testing"
)

func TestAtmAdd(t *testing.T) {
	var a AtmI64
	a.Add(1)
	a.Add(1)
	if a != 2 {
		t.Fatalf("RecvMsgCnt != 2,RecvMsgCnt=%d", a)
	}
}
