package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/config"
)

var mux = http.NewServeMux()

func router() {
	mux.HandleFunc(config.DashboordApiUrl+"/overview", Overview.Get)
	mux.HandleFunc(config.DashboordApiUrl+"/log", SetLogLevel)
	mux.HandleFunc(config.DashboordApiUrl+"/broker", GetSessions)
}

func run() {
	router()
	s := http.Server{}
	s.Addr = fmt.Sprintf(":%d", config.RestfulPort)
	s.Handler = mux
	go func() {
		log.Debug("dashboad http started")
		log.Println(s.ListenAndServe())
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
				Overview.SentMsgPerSeconds.Set(int64(rpsTotall / perCnt))
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

	d := <-this.outChan
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
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
	log.Debug("dashboard.GetSessions")

	list := sessionMgr.GetSessions()
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	encoder.Encode(list)
}
