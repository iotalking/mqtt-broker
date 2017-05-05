package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"

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

var listenNewChan = make(chan net.Listener)

var wgExit sync.WaitGroup

var basePort = 1883

func mqttServer(r *runtine.SafeRuntine, args ...interface{}) {

	wgExit.Add(1)
	defer func() {
		wgExit.Done()
	}()
	log.Debugf("mqttServer args:%#v", args)
	addr := fmt.Sprintf(":%d", basePort+args[0].(int))
	l, err := net.Listen("tcp", addr)

	if err != nil {
		panic(err)
	}
	if len(dashboard.Overview.BrokerAddress) == 0 {
		dashboard.Overview.BrokerAddress = l.Addr().String()
		log.Info("brokerAddress:", dashboard.Overview.BrokerAddress)
	}
	listenNewChan <- l
	for {
		c, err := l.Accept()

		select {
		case <-r.IsInterrupt:
			log.Debugln("listener was instructed to quit")
			return
		default:
		}
		if err != nil {
			log.Error("server accept error.", err)
			break
		} else {
			if c == nil {
				panic("Accept conn is nil")
			}
			dashboard.Overview.InactiveClients.Add(1)
			dashboard.Overview.OpenedFiles.Add(1)
			if dashboard.Overview.OpenedFiles > dashboard.Overview.MaxOpenedFiles {
				dashboard.Overview.MaxOpenedFiles.Set(int64(dashboard.Overview.OpenedFiles))
			}
			sessionMgr.HandleConnection(c)
		}

	}
}

var sessionMgr = session.GetMgr()

func main() {
	level := flag.String("log", "debug", "log level string")
	listenMax := flag.Int("listenMax", 1, "max of listen port")
	flag.Parse()
	loglevel, err := log.ParseLevel(*level)
	if err != nil {
		panic(err)
	}
	log.SetLevel(loglevel)

	log.Debugln("server running")

	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()

	var closeAllListenChan = make(chan bool)
	var listens []net.Listener

	go func() {
		for {
			select {
			case l := <-listenNewChan:
				listens = append(listens, l)
			case <-closeAllListenChan:
				for _, l := range listens {
					l.Close()
				}
				closeAllListenChan <- true
			}
		}
	}()
	dashboard.Init(sessionMgr)

	for i := 0; i < *listenMax; i++ {
		runtine.Go(mqttServer, i)
	}

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt)

	select {
	case <-signals:
		//关闭服务器的端口监听，以退出
		closeAllListenChan <- true
		<-closeAllListenChan
		wgExit.Wait()
	}
	log.Debugf("server stoped:%#v", sessionMgr)
}
