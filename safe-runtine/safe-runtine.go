package runtine

import (
	"sync"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
)

type SafeRuntine struct {
	startedWG sync.WaitGroup
	stopedWG  sync.WaitGroup

	//SafeRuntine的运行函数里可以通过接收IsInterrupt chan来判断是否要安全退出
	//调用都可以调用Stop便可以安全退出
	//例如:
	//runtine.Go(func(r *runtine.SafeRuntine){
	//		select{
	//			case <-r.IsInterrupt:
	//		}
	//)
	//
	IsInterrupt chan struct{}

	stoped int32
}

func Go(fn func(r *SafeRuntine)) *SafeRuntine {
	var o SafeRuntine
	o.startedWG.Add(1)
	o.IsInterrupt = make(chan struct{})
	go func() {
		atomic.AddInt32(&o.stoped, 1)
		o.startedWG.Done()
		o.stopedWG.Add(1)
		fn(&o)
		o.stopedWG.Done()
		atomic.StoreInt32(&o.stoped, 0)
	}()

	return &o
}

func (this *SafeRuntine) Stop() {

	if atomic.LoadInt32(&this.stoped) == 0 {
		log.Println("runtine has exit")
		return
	}
	log.Debugln("runtine want to stop")
	close(this.IsInterrupt)

	this.stopedWG.Wait()
	log.Debugln("runtine want to stoped")
	return
}

func (this *SafeRuntine) IsStoped() bool {
	return atomic.LoadInt32(&this.stoped) == 0
}
