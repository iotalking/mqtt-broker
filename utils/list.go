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
		ch: make(chan byte, 1),
	}
}

func (this *List) Push(v interface{}) {
	if v == nil {
		panic(nil)
	}
	this.mux.Lock()
	this.len++
	this.l.PushBack(v)
	this.mux.Unlock()

	select {
	case this.ch <- 0:
	default:
	}
}
func (this *List) Pop() interface{} {
	this.mux.Lock()
	defer this.mux.Unlock()

	if this.l.Len() > 0 {
		v := this.l.Remove(this.l.Front())
		if this.l.Len() > 0 {
			select {
			case this.ch <- 0:
			default:
			}
		}

		return v
	}
	return nil
}
func (this *List) Len() int {
	this.mux.Lock()
	defer this.mux.Unlock()
	return this.l.Len()
}

//遍历列表
//cb返回true时停止遍历，false继续遍历
func (this *List) Each(cb func(v interface{}) (stop bool)) {
	this.mux.Lock()
	defer this.mux.Unlock()
	for f := this.l.Front(); f != nil; f = f.Next() {
		stop := cb(f.Value)
		if stop {
			break
		}
	}
}

//删除元素
//cb返回true时删除当前元素
func (this *List) Remove(cb func(v interface{}) (del, c bool)) {
	this.mux.Lock()
	defer this.mux.Unlock()
	for f := this.l.Front(); f != nil; {
		del, _continue := cb(f.Value)
		if del {
			n := f.Next()
			this.l.Remove(f)
			f = n
		} else {
			f = f.Next()
		}
		if !_continue {
			break
		}
	}
}

func (this *List) Wait() <-chan byte {
	return this.ch
}
