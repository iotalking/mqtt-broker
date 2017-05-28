package dashboard

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type AtmType interface {
	Set(v interface{})
	Get() interface{}
}

//线程安全64位整数
type AtmI64 int64

func (o *AtmI64) Add(v int64) {
	atomic.AddInt64((*int64)(o), v)
}
func (o *AtmI64) Set(v interface{}) {
	atomic.StoreInt64((*int64)(o), v.(int64))
}

func (o *AtmI64) Get() (v interface{}) {
	return atomic.LoadInt64((*int64)(o))
}

type AtmString struct {
	v   string
	mtx sync.Mutex
}

func (o *AtmString) String() string {
	return o.Get().(string)
}
func (o *AtmString) Add(v interface{}) {
}
func (o *AtmString) Set(v interface{}) {
	o.mtx.Lock()
	o.v = v.(string)
	o.mtx.Unlock()
}
func (o *AtmString) Get() (v interface{}) {
	o.mtx.Lock()
	v = o.v
	o.mtx.Unlock()
	return
}
func (o *AtmString) MarshalJSON() ([]byte, error) {
	v := o.Get()
	return []byte(fmt.Sprintf(`"%s"`, v.(string))), nil
}
func (o *AtmString) UnmarshalJSON(v []byte) (err error) {
	var s string

	l := len(v)
	if l > 1 {
		s = string(v[1 : l-1])
	}
	o.Set(s)
	return nil
}
