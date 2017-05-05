package session

import (
	"net"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/safe-runtine"
	"github.com/iotalking/mqtt-broker/topic"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/iotalking/mqtt-broker/dashboard"
)

type SessionMgr struct {
	//主runtine
	runtine *runtine.SafeRuntine

	//用于处理新连接
	newConnChan chan net.Conn
	//连接验证超时
	connectTimeoutChan chan *Session
	//连接验证通过
	connectedChan chan *Session
	//已经连接难的session主动断开连接
	disconnectChan chan *Session

	//保存未验证连接的session
	waitingConnectSessionMap map[net.Conn]*Session

	//保存已经验证连接的session
	connectedSessionMap map[string]*Session

	subscriptionMgr *topic.SubscriptionMgr

	getSessionsChan chan byte
	//返回json的active session和inactive session的clientId列表
	getSessionsResultChan chan dashboard.SessionList

	getActiveSessionsChan       chan byte
	getActiveSessionsResultChan chan []*Session

	getAllSessionChan       chan byte
	getAllSessionResultChan chan []*Session

	closeSessionRuntine *runtine.SafeRuntine
	closeSessionChan    chan *Session

	tickerRuntine *runtine.SafeRuntine
}

var sessionMgr *SessionMgr
var sessionMgrOnce sync.Once

func GetMgr() *SessionMgr {
	sessionMgrOnce.Do(func() {
		mgr := &SessionMgr{
			connectedSessionMap:         make(map[string]*Session),
			newConnChan:                 make(chan net.Conn),
			subscriptionMgr:             topic.NewSubscriptionMgr(),
			connectTimeoutChan:          make(chan *Session),
			connectedChan:               make(chan *Session),
			disconnectChan:              make(chan *Session),
			waitingConnectSessionMap:    make(map[net.Conn]*Session),
			getSessionsChan:             make(chan byte),
			getSessionsResultChan:       make(chan dashboard.SessionList),
			getActiveSessionsChan:       make(chan byte),
			getActiveSessionsResultChan: make(chan []*Session),
			getAllSessionChan:           make(chan byte),
			getAllSessionResultChan:     make(chan []*Session),
			closeSessionChan:            make(chan *Session),
		}
		runtine.Go(func(r *runtine.SafeRuntine, args ...interface{}) {
			mgr.runtine = r
			mgr.run()
		})
		runtine.Go(func(r *runtine.SafeRuntine, args ...interface{}) {
			mgr.closeSessionRuntine = r
			for {
				select {
				case <-r.IsInterrupt:

					return
				case s := <-mgr.closeSessionChan:
					log.Info("sessionMgr closing session")

					s.Close()
					dashboard.Overview.ClosingFiles.Add(-1)
					log.Infof("sessionMgr has closed session:%s", s.clientId)

				}
			}
			log.Info("closeSessionRuntine is closed")
		})
		runtine.Go(func(r *runtine.SafeRuntine, args ...interface{}) {
			mgr.tickerRuntine = r
			mgr.tickerRun()
		})
		sessionMgr = mgr

	})

	return sessionMgr
}

func (this *SessionMgr) Close() {
	if this.runtine.IsStoped() {
		log.Warnf("Close:sessionMgr istoped")
		return
	}
	this.tickerRuntine.Stop()
	this.closeSessionRuntine.Stop()
	this.runtine.Stop()
}

func (this *SessionMgr) CloseSession(s *Session) {
	dashboard.Overview.ClosingFiles.Add(1)
	this.closeSessionChan <- s
}
func (this *SessionMgr) HandleConnection(c net.Conn) {
	if this.runtine.IsStoped() {
		log.Warnf("sessionMgr istoped")
		return
	}

	this.newConnChan <- c
}
func (this *SessionMgr) tickerRun() {
	var secondTicker = time.NewTimer(time.Second)
	for {
		select {
		case <-secondTicker.C:
			preTime := time.Now()
			asessions := this.getAllSesssions()

			for _, s := range asessions {
				s.OnTick()
			}

			usedNs := time.Now().Sub(preTime).Nanoseconds()
			d := time.Second - time.Duration(usedNs)
			log.Info("ontick used:", usedNs)
			secondTicker.Reset(d)
		}

	}
}
func (this *SessionMgr) run() {
	for {
		select {
		case <-this.runtine.IsInterrupt:
			break

		case c := <-this.newConnChan:
			log.Debugf("newConnChan got a conn.%s", c.RemoteAddr().String())
			session := NewSession(this, c, true)
			this.waitingConnectSessionMap[c] = session
			dashboard.Overview.InactiveClients.Set(int64(len(this.waitingConnectSessionMap)))
		case s := <-this.connectTimeoutChan:
			delete(this.waitingConnectSessionMap, s.channel.conn)
			dashboard.Overview.InactiveClients.Set(int64(len(this.waitingConnectSessionMap)))
			this.CloseSession(s)
		case s := <-this.connectedChan:
			log.Infof("session %s connected", s.clientId)
			delete(this.waitingConnectSessionMap, s.channel.conn)
			this.connectedSessionMap[s.clientId] = s
			dashboard.Overview.ActiveClients.Set(int64(len(this.connectedSessionMap)))
			dashboard.Overview.InactiveClients.Set(int64(len(this.waitingConnectSessionMap)))
		case s := <-this.disconnectChan:
			log.Info("sessionMgr disconnet client:", s.clientId)
			delete(this.connectedSessionMap, s.clientId)
			//由session mgr来安全退出session,因为是由mgr创建的
			dashboard.Overview.ActiveClients.Set(int64(len(this.connectedSessionMap)))
			this.CloseSession(s)
		case <-this.getSessionsChan:
			log.Debug("sessionMgr.getSessionsChan")
			list := dashboard.SessionList{}
			for _, s := range this.connectedSessionMap {
				list.Active = append(list.Active, s.clientId)
			}
			for _, s := range this.waitingConnectSessionMap {
				list.Inactive = append(list.Inactive, s.clientId)
			}
			select {
			case this.getSessionsResultChan <- list:
			default:

			}
		case <-this.getActiveSessionsChan:
			log.Debug("sessionMgr.getSessionsChan")
			sessions := make([]*Session, 0, len(this.connectedSessionMap))
			for _, s := range this.connectedSessionMap {

				sessions = append(sessions, s)
			}
			this.getActiveSessionsResultChan <- sessions
		case <-this.getAllSessionChan:
			sessions := make([]*Session, 0, len(this.connectedSessionMap))
			for _, s := range this.waitingConnectSessionMap {
				sessions = append(sessions, s)
			}
			for _, s := range this.connectedSessionMap {
				sessions = append(sessions, s)
			}
			this.getAllSessionResultChan <- sessions
		}
	}
}

type mgrPublishData struct {
	msg     *packets.PublishPacket
	session *Session
}

func (this *SessionMgr) OnConnected(session *Session) {
	//不能阻塞session，不然会死锁
	this.connectedChan <- session

}

func (this *SessionMgr) OnDisconnected(session *Session) {
	//不能阻塞session，不然会死锁
	this.disconnectChan <- session
}

//连接验证超时
func (this *SessionMgr) OnConnectTimeout(session *Session) {
	//不能阻塞session，不然会死锁
	this.connectTimeoutChan <- session
}

func (this *SessionMgr) OnSubscribe(msg *packets.SubscribePacket, session *Session) {

}

func (this *SessionMgr) OnUnsubscribe(msg *packets.UnsubscribePacket, session *Session) {

}

func (this *SessionMgr) GetSessions() dashboard.SessionList {
	log.Debug("SessionMgr.GetSessions")
	this.getSessionsChan <- 1
	return <-this.getSessionsResultChan
}

func (this *SessionMgr) getActiveSessions() []*Session {
	this.getActiveSessionsChan <- 1
	return <-this.getActiveSessionsResultChan
}

func (this *SessionMgr) getAllSesssions() []*Session {
	this.getAllSessionChan <- 1
	return <-this.getAllSessionResultChan
}
