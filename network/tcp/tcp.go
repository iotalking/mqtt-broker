package tcp

import (
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/session"
)

var listener net.Listener

func Start(addr string) {
	var err error
	listener, err = net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	if len(dashboard.Overview.BrokerAddress) == 0 {
		dashboard.Overview.BrokerAddress = listener.Addr().String()
		log.Info("brokerAddress:", dashboard.Overview.BrokerAddress)
	}
	var sessionMgr = session.GetMgr()
	for {
		c, err := listener.Accept()

		if err != nil {
			log.Error("server accept error.", err)
			break
		} else {
			if c == nil {
				panic("tcp accept a nil conn")
			}
			if dashboard.Overview.OpenedFiles > dashboard.Overview.MaxOpenedFiles {
				dashboard.Overview.MaxOpenedFiles.Set(int64(dashboard.Overview.OpenedFiles))
			}
			sessionMgr.HandleConnection(c)
		}

	}
	return
}

func Stop() {
	listener.Close()
}
