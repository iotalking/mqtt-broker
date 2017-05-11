package websocket

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
)

func StartTLS(addr string, certFile, keyFile string) {
	log.Infof("starting websocket tls on :%s", addr)
	mux.HandleFunc("/mqtts", serverWS)
	s := http.Server{}
	s.Addr = addr
	s.Handler = mux
	err := s.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		panic(err)
	}
}
