package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/config"
)

var mux = http.NewServeMux()

func router() {
	mux.HandleFunc(config.DashboordApiUrl+"/overview", Overview.Get)
	mux.HandleFunc(config.DashboordApiUrl+"/log", SetLogLevel)
	mux.HandleFunc(config.DashboordApiUrl+"/activeSessions", GetSessions)

	mux.HandleFunc(config.DashboordUrl+"/client", svrFile("./dashboard/www/client/index.html"))
	mux.HandleFunc(config.DashboordUrl+"/client/browserMqtt.js", svrFile("./dashboard/www/client/browserMqtt.js"))
	mux.HandleFunc(config.DashboordUrl+"/clientls", svrFile("./dashboard/www/client/tls.html"))

}

func run(addr string) {
	router()
	s := http.Server{}
	s.Addr = addr
	s.Handler = mux
	go func() {
		log.Debugf("dashboad http started on :%s", addr)
		err := s.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
	duration := time.Second
	var _secondsTimer = time.NewTimer(duration)
	var lastSentMsgCnt = Overview.SentMsgCnt
	var lastRecvMsgCnt = Overview.RecvMsgCnt
	var lastPerTime = time.Now()
	var perCnt AtmI64
	var spsTotall AtmI64
	var rpsTotall AtmI64

	var lastSentBytes = Overview.SentBytes
	var lastRecvBytes = Overview.RecvBytes
	//平均发送速率总和
	var sbpsTotall AtmI64
	//平均接收速率总和
	var rbpsTotall AtmI64
	for {
		select {
		case <-_secondsTimer.C:
			//每秒计算一次平均数
			tm := time.Now().Sub(lastPerTime).Seconds()

			sps := (Overview.SentMsgCnt - lastSentMsgCnt) / AtmI64(tm)
			rps := (Overview.RecvMsgCnt - lastRecvMsgCnt) / AtmI64(tm)

			sbps := (Overview.SentBytes - lastSentBytes) / AtmI64(tm)
			rbps := (Overview.RecvBytes - lastRecvBytes) / AtmI64(tm)

			perCnt++
			if sps > 0 {
				spsTotall += sps
				Overview.SentMsgPerSeconds.Set(int64(spsTotall / perCnt))
			}
			if rps > 0 {
				rpsTotall += rps
				Overview.RecvMsgPerSeconds.Set(int64(rpsTotall / perCnt))
			}
			if sbps > 0 {
				sbpsTotall += sbps
				Overview.SentBytesPerSeconds.Set(int64(rbpsTotall / perCnt))
			}
			if rbps > 0 {
				rbpsTotall += rbps
				Overview.RecvBytesPerSeconds.Set(int64(rbpsTotall / perCnt))
			}
			lastPerTime = time.Now()
			lastSentMsgCnt = Overview.SentMsgCnt
			lastRecvMsgCnt = Overview.RecvMsgCnt

			lastSentBytes = Overview.SentBytes
			lastRecvBytes = Overview.RecvBytes

			_secondsTimer.Reset(duration)
		case <-Overview.getChan:
			log.Debug("geting overview data")
			Overview.outChan <- *Overview
		}
	}
}
func (this *OverviewData) Get(w http.ResponseWriter, r *http.Request) {
	tm := time.Now().Sub(startTime)
	Overview.RunTimeString = tm.String()
	Overview.RunNanoSeconds.Set(tm.Nanoseconds())
	log.Debug("OverviewData.Get")
	select {
	case this.getChan <- 0:
	default:
	}
	name := r.FormValue("name")

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	d := <-this.outChan
	if len(name) > 0 {
		log.Debug("name = ", name)
		value := reflect.ValueOf(Overview).Elem().FieldByName(r.FormValue("name"))
		if value.IsValid() {

			encoder.Encode(fmt.Sprintf("%v", value))

			log.Info("OverviewData.Get name=", value)
			return
		} else {
			log.Debug("no field of ", name)
		}
	} else {
		log.Debug("name is empty")
	}
	encoder.Encode(d)

}

func SetLogLevel(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			http.Error(w, "level=<panic|fatal|error|warn|warning|info|debug>", http.StatusBadRequest)
		}
	}()
	err = r.ParseForm()
	if err != nil {
		log.Error("dashboard.SetLogLevel error:", err)
		return
	}
	v, err := log.ParseLevel(r.FormValue("level"))
	if err != nil {
		log.Error("dashboard.ParseLevel error:", err)
		return
	}
	log.SetLevel(v)
}

func GetSessions(w http.ResponseWriter, r *http.Request) {
	log.Debug("dashboard.GetActiveSessions")

	list := sessionMgr.GetSessions()
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	encoder.Encode(list)
}

func svrFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("srv File:%s", name)

		http.ServeFile(w, r, name)
	}

}
