package main

import (
	"net"
	"os"
	"os/signal"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/safe-runtine"
	"github.com/iotalking/mqtt-broker/session"

	"net/http"
	_ "net/http/pprof"
)

func init() {
	//	os.OpenFile("./log.txt", os.O_CREATE|os.O_WRONLY, 666)
	log.SetOutput(os.Stdout)
	//	log.SetLevel(log.WarnLevel)
	log.SetLevel(log.DebugLevel)
}

func main() {

	var sessionMgr = session.GetMgr()

	l, err := net.Listen("tcp", ":1883")

	if err != nil {
		panic(err)
	}
	log.Debugln("server running")

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	dashboard.Init(sessionMgr)

	mainRuntine := runtine.Go(func(r *runtine.SafeRuntine) {
		for {
			c, err := l.Accept()

			select {
			case <-r.IsInterrupt:
				log.Debugln("listener was instructed to quit")
				return
			default:
			}
			if err != nil {
				log.Debugln("server accept error.", err)
			}

			sessionMgr.HandleConnection(c)
		}
	})

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt)

	select {
	case <-signals:
		//关闭服务器的端口监听，以退出
		l.Close()
		mainRuntine.Stop()
	}
	log.Debugf("server stoped:%#v", sessionMgr)
}
