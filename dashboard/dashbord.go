package dashboard

import (
	"sync/atomic"
)

//线程安全64位整数
type AtmI64 int64

func (o *AtmI64) Add(v int64) {
	atomic.AddInt64((*int64)(o), v)
}
func (o *AtmI64) Set(v int64) {
	atomic.StoreInt64((*int64)(o), v)
}
func (o *AtmI64) Get() (v int64) {
	return atomic.LoadInt64((*int64)(o))
}
