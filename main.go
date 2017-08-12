package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	pprof "runtime/pprof"
	"runtime/trace"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/config"
	"github.com/iotalking/mqtt-broker/network/tcp"
	"github.com/iotalking/mqtt-broker/network/websocket"
	"github.com/iotalking/mqtt-broker/restful_api"
	"github.com/iotalking/mqtt-broker/session"
	"github.com/iotalking/mqtt-broker/www"

	"net/http"
	_ "net/http/pprof"

	_ "github.com/iotalking/mqtt-broker/store/mem-provider"
)

type timeFormater struct {
	log.TextFormatter
}

func (this timeFormater) Format(e *log.Entry) ([]byte, error) {
	bs, err := this.TextFormatter.Format(e)
	bs = []byte(fmt.Sprintf("%d: %s", e.Time.UnixNano(), bs))
	return bs, err
}

func init() {
	//	os.OpenFile("./log.txt", os.O_CREATE|os.O_WRONLY, 666)
	log.SetOutput(os.Stdout)
	//	log.SetLevel(log.WarnLevel)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(timeFormater{})
}

var listenNewChan = make(chan net.Listener)

var basePort = 1883

var sessionMgr = session.GetMgr()
var level = flag.String("log", "debug", "log level string")
var traceFileName = flag.String("trace", "", "out trace to file")
var cpuprofile = flag.String("cpuprofile", "", "CPU Profile Filename")

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
	}

	if len(*traceFileName) > 0 {
		f, err := os.OpenFile(*traceFileName, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			panic(err)
		}
		trace.Start(f)
		defer trace.Stop()
	}

	loglevel, err := log.ParseLevel(*level)
	if err != nil {
		panic(err)
	}
	log.SetLevel(loglevel)

	log.Debugln("server starting")
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wg.Done()
		log.Debug("starting tcp server...")
		tcp.Start(fmt.Sprintf(":%d", config.TcpPort))
	}()
	wg.Add(1)
	go func() {
		wg.Done()
		log.Debug("starting tcp tls server")
		tcp.StartTLS(fmt.Sprintf(":%d", config.TcpTLSPort), config.CertFileName, config.KeyFileName)
	}()
	wg.Add(1)

	wg.Add(1)
	go func() {
		wg.Done()
		log.Debug("starting websocket server...")
		websocket.Start(fmt.Sprintf(":%d", config.WSPort))
	}()

	wg.Add(1)
	go func() {
		wg.Done()
		log.Debug("starting websocket tls server...")
		websocket.StartTLS(fmt.Sprintf(":%d", config.WSSPort), config.CertFileName, config.KeyFileName)
	}()

	go func() {
		wg.Done()
		log.Debug("starting restful api server...")
		api.Start(fmt.Sprintf(":%d", config.RestfulPort), sessionMgr)
	}()

	wg.Add(1)
	go func() {
		wg.Done()
		log.Debug("starting debug pprof server...")
		err = http.ListenAndServe(fmt.Sprintf(":%d", config.PprofPort), nil)
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		wg.Done()
		log.Debugf("starting www server on:%d ...", config.WebPort)
		err = www.Start(fmt.Sprintf(":%d", config.WebPort))
		if err != nil {
			panic(err)
		}
	}()

	wg.Wait()
	log.Debugln("server running")
	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt)

	select {
	case <-signals:
		//关闭服务器的端口监听，以退出
		tcp.Stop()
		tcp.StopTLS()
	}

	log.Debugf("server stoped:%#v", sessionMgr)
}
