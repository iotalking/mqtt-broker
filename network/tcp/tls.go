package tcp

import (
	"crypto/tls"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/session"
)

var tlsListener net.Listener

func StartTLS(addr string, certFile, keyFile string) {
	var config tls.Config
	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		panic(err)
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	tlsListener = tls.NewListener(l, &config)

	log.Infof("tcp tls started on :%s", addr)
	var sessionMgr = session.GetMgr()
	for {
		c, err := tlsListener.Accept()

		if err != nil {
			log.Error("server accept error.", err)
			break
		} else {
			if c == nil {
				panic("tls accept nil conn")
			}
			log.Error("tls accept a conn")
			if dashboard.Overview.OpenedFiles > dashboard.Overview.MaxOpenedFiles {
				dashboard.Overview.MaxOpenedFiles.Set(int64(dashboard.Overview.OpenedFiles))
			}
			sessionMgr.HandleConnection(session.NewSession(sessionMgr, c, true))
		}

	}
	return
}

func StopTLS() {
	if tlsListener == nil {
		panic(nil)
	}
	tlsListener.Close()
	tlsListener = nil
}
