package ticker

import (
	"sync"
	"time"

	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/utils"
)

type Ticker interface {
	Tick()
}

var (
	secondsOnce sync.Once
	//保存未到基的ticker
	secondsTickMap = make(map[time.Time]*tickData)

	afterList  = utils.NewList()
	cancelList = utils.NewList()

	dataPool = sync.Pool{
		New: func() interface{} {
			return &tickData{}
		},
	}
)

type tickData struct {
	id     time.Time
	ticker Ticker
}

type Timer struct {
	ch    chan interface{}
	id    time.Time
	param interface{}
	mux   sync.Mutex
}

func NewTimer(ch chan interface{}) *Timer {
	secondsOnce.Do(func() {
		go run()
	})
	if ch == nil {
		ch = make(chan interface{})
	}
	t := &Timer{
		ch: ch,
	}

	return t
}
func (this *Timer) Reset(duration time.Duration, param interface{}) {
	cancelTicker(this.id)
	this.mux.Lock()
	this.param = param
	this.mux.Unlock()
	this.id = after(duration, this)
}
func (this *Timer) Stop() {
	cancelTicker(this.id)

	this.mux.Lock()
	this.param = nil
	this.ch = nil
	this.mux.Unlock()
}

func (this *Timer) Wait() <-chan interface{} {
	return this.ch
}
func (this *Timer) Tick() {
	this.mux.Lock()
	select {
	case this.ch <- this.param:
	default:
	}
	this.mux.Unlock()
}

//延时duration后，调用tk的Tick
//运行一个常驻的runntine,再runtine中启动一个一秒的定时器，每秒循环比较每个ticker到期时间戳,
//不用要time.AfterFunc一样每次的都启动一个runtine
//time.After又每次都产生一个新的chan,不好select
func after(duration time.Duration, tk Ticker) time.Time {

	tm := time.Now()
	id := tm.Add(duration)
	var data *tickData

	data = dataPool.Get().(*tickData)
	data.id = id
	data.ticker = tk

	afterList.Push(data)
	return id
}

func cancelTicker(tm time.Time) {
	cancelList.Push(tm)
}

func run() {
	//用time来做延时,每隔一秒检查一下定时器列表

	var d time.Duration = time.Second
	for {
		select {
		case <-time.After(d):
			last := time.Now()
			//遍历tickers
			//如果超时则调用Tick
			dashboard.Overview.TickerMapCnt.Set(int64(len(secondsTickMap)))

			for tm, data := range secondsTickMap {
				if tm.Sub(last) < 0 {
					//ticker小于当前时间，表示超时了
					data.ticker.Tick()
					delete(secondsTickMap, tm)
					dataPool.Put(data)
				}

			}
			//下一次定时器要减去遍历使用的时间
			d = time.Second - time.Since(last)
			if d < 0 {
				d = 0
			}
		case <-afterList.Wait():
			dashboard.Overview.AddTickerCnt.Add(1)
			data := afterList.Pop().(*tickData)
			secondsTickMap[data.id] = data

		case <-cancelList.Wait():
			id := cancelList.Pop().(time.Time)
			if data, ok := secondsTickMap[id]; ok {
				delete(secondsTickMap, id)
				dataPool.Put(data)
			}
		}
	}
}
