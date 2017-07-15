package api

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/iotalking/mqtt-broker/dashboard/api"
	"github.com/iotalking/mqtt-broker/session"
)

func Start(addr string, mgr session.SessionMgr) {
	s := http.Server{}
	s.Addr = addr
	s.Handler = mux
	router()
	go func() {
		api.Start(mgr)
	}()
	log.Debugf("restfull  http started on :%s", addr)
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
