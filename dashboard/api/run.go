package api

import (
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/session"
)

type SessionMgr interface {
	GetSessions() session.SessionList
}

var sessionMgr SessionMgr
var startTime time.Time
var getChan chan byte
var outChan chan dashboard.OverviewData

func Start(addr string, mgr SessionMgr) {
	startTime = time.Now()

	sessionMgr = mgr
	getChan = make(chan byte)
	outChan = make(chan dashboard.OverviewData)
	run(addr)
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
	var lastSentMsgCnt = dashboard.Overview.SentMsgCnt
	var lastRecvMsgCnt = dashboard.Overview.RecvMsgCnt
	var lastPerTime = time.Now()
	var perCnt dashboard.AtmI64
	var spsTotall dashboard.AtmI64
	var rpsTotall dashboard.AtmI64

	var lastSentBytes = dashboard.Overview.SentBytes
	var lastRecvBytes = dashboard.Overview.RecvBytes
	//平均发送速率总和
	var sbpsTotall dashboard.AtmI64
	//平均接收速率总和
	var rbpsTotall dashboard.AtmI64
	for {
		select {
		case <-_secondsTimer.C:
			//每秒计算一次平均数
			tm := time.Now().Sub(lastPerTime).Seconds()

			sps := (dashboard.Overview.SentMsgCnt - lastSentMsgCnt) / dashboard.AtmI64(tm)
			rps := (dashboard.Overview.RecvMsgCnt - lastRecvMsgCnt) / dashboard.AtmI64(tm)

			sbps := (dashboard.Overview.SentBytes - lastSentBytes) / dashboard.AtmI64(tm)
			rbps := (dashboard.Overview.RecvBytes - lastRecvBytes) / dashboard.AtmI64(tm)

			perCnt++
			if sps > 0 {
				spsTotall += sps
				dashboard.Overview.SentMsgPerSeconds.Set(int64(spsTotall / perCnt))
			}
			if rps > 0 {
				rpsTotall += rps
				dashboard.Overview.RecvMsgPerSeconds.Set(int64(rpsTotall / perCnt))
			}
			if sbps > 0 {
				sbpsTotall += sbps
				dashboard.Overview.SentBytesPerSeconds.Set(int64(rbpsTotall / perCnt))
			}
			if rbps > 0 {
				rbpsTotall += rbps
				dashboard.Overview.RecvBytesPerSeconds.Set(int64(rbpsTotall / perCnt))
			}
			lastPerTime = time.Now()
			lastSentMsgCnt = dashboard.Overview.SentMsgCnt
			lastRecvMsgCnt = dashboard.Overview.RecvMsgCnt

			lastSentBytes = dashboard.Overview.SentBytes
			lastRecvBytes = dashboard.Overview.RecvBytes

			_secondsTimer.Reset(duration)
		}
	}
}
