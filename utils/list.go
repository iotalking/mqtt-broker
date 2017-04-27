package utils

import (
	"container/list"
	"sync"
)

type List struct {
	l   *list.List
	len int
	mux sync.Mutex
	ch  chan byte
}

func NewList() *List {
	return &List{
		l:  list.New(),
		ch: make(chan byte),
	}
}

func (this *List) Push(v interface{}) {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.len++
	this.l.PushBack(v)
	select {
	case this.ch <- 0:
	default:
	}
}
func (this *List) Pop() interface{} {
	this.mux.Lock()
	defer this.mux.Unlock()
	if this.l.Len() > 0 {
		select {
		case this.ch <- 0:
		default:
		}

		return this.l.Remove(this.l.Front())
	}
	return nil
}
func (this *List) Len() int {
	this.mux.Lock()
	defer this.mux.Unlock()
	return this.l.Len()
}

func (this *List) Wait() <-chan byte {
	return this.ch
}
