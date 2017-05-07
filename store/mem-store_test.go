package store

import (
	"reflect"
	"testing"
)

func TestError(t *testing.T) {
	e := ErrDupMsg

	t.Log(e)
}
