package session

import (
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/safe-runtine"
	"github.com/iotalking/mqtt-broker/topic"
	"github.com/iotalking/mqtt-broker/utils"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/iotalking/mqtt-broker/dashboard"
)

type SessionMgr struct {
	//主runtine
	runtine *runtine.SafeRuntine
	//消息分布runtine
	publishRuntine *runtine.SafeRuntine

	//用于处理新连接
	newConnList *utils.List
	//连接验证超时
	connectTimeoutList *utils.List
	//连接验证通过
	connectedList *utils.List
	//已经连接难的session主动断开连接
	disconnectList *utils.List

	//保存未验证连接的session
	waitingConnectSessionMap map[net.Conn]*Session

	//保存已经验证连接的session
	connectedSessionMap map[string]*Session

	subscriptionMgr *topic.SubscriptionMgr

	//保存要发布的消息
	publishList *utils.List
	//循环使用mgrPublishData对象，减少gc时间
	publishDataPool *sync.Pool

	getSessionsChan chan byte
	//返回json的active session和inactive session的clientId列表
	getSessionsResultChan chan dashboard.SessionList

	getActiveSessionsChan       chan byte
	getActiveSessionsResultChan chan []*Session
}

var sessionMgr *SessionMgr
var sessionMgrOnce sync.Once

func GetMgr() *SessionMgr {
	sessionMgrOnce.Do(func() {
		mgr := &SessionMgr{
			connectedSessionMap:      make(map[string]*Session),
			newConnList:              utils.NewList(),
			subscriptionMgr:          topic.NewSubscriptionMgr(),
			connectTimeoutList:       utils.NewList(),
			connectedList:            utils.NewList(),
			disconnectList:           utils.NewList(),
			waitingConnectSessionMap: make(map[net.Conn]*Session),
			publishList:              utils.NewList(),
			publishDataPool: &sync.Pool{
				New: func() interface{} {
					return &mgrPublishData{}
				},
			},
			getSessionsChan:             make(chan byte),
			getSessionsResultChan:       make(chan dashboard.SessionList),
			getActiveSessionsChan:       make(chan byte),
			getActiveSessionsResultChan: make(chan []*Session),
		}
		runtine.Go(func(r *runtine.SafeRuntine) {
			mgr.runtine = r
			mgr.run()
		})
		runtine.Go(func(r *runtine.SafeRuntine) {
			mgr.publishRuntine = r
			mgr.publishRun()
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
	this.publishRuntine.Stop()
	this.runtine.Stop()
}
func (this *SessionMgr) HandleConnection(c net.Conn) {
	if this.runtine.IsStoped() {
		log.Warnf("sessionMgr istoped")
		return
	}

	this.newConnList.Push(c)
}

func (this *SessionMgr) run() {
	for {
		select {
		case <-this.runtine.IsInterrupt:
			break
		case <-this.newConnList.Wait():
			c := this.newConnList.Pop().(net.Conn)

			log.Debugf("newConnChan got a conn.%s", c.RemoteAddr().String())
			session := NewSession(this, c, true)
			this.waitingConnectSessionMap[c] = session
			dashboard.Overview.InactiveClients.Set(int64(len(this.waitingConnectSessionMap)))
		case <-this.connectTimeoutList.Wait():
			s := this.connectTimeoutList.Pop().(*Session)
			delete(this.waitingConnectSessionMap, s.channel.conn)
			s.Close()
			dashboard.Overview.InactiveClients.Set(int64(len(this.waitingConnectSessionMap)))
		case <-this.connectedList.Wait():
			s := this.connectedList.Pop().(*Session)
			delete(this.waitingConnectSessionMap, s.channel.conn)
			this.connectedSessionMap[s.clientId] = s
			dashboard.Overview.ActiveClients.Set(int64(len(this.connectedSessionMap)))
			dashboard.Overview.InactiveClients.Set(int64(len(this.waitingConnectSessionMap)))
		case <-this.disconnectList.Wait():
			s := this.disconnectList.Pop().(*Session)
			log.Error("sessionMgr disconnet client:", s.clientId)
			delete(this.connectedSessionMap, s.clientId)
			//由session mgr来安全退出session,因为是由mgr创建的
			log.Errorf("disconnecting count:%d", this.disconnectList.Len())
			s.Close()
			dashboard.Overview.ActiveClients.Set(int64(len(this.connectedSessionMap)))

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
		}
	}
}

type mgrPublishData struct {
	msg     *packets.PublishPacket
	session *Session
}

func (this *SessionMgr) publishRun() {
	for {
		select {
		case <-this.publishList.Wait():
			//发送最前的消息
			for {
				v := this.publishList.Pop()
				if v != nil {
					data := v.(*mgrPublishData)
					sessions := this.getActiveSessions()
					for _, s := range sessions {
						log.Debugf("%#v", s)
						if s.IsConnected() {
							s.Publish(data.msg)
						}
					}

					this.publishDataPool.Put(data)
				} else {
					break
				}
			}

		}

	}
}

//向指定主题广播消息
//非阻塞
func (this *SessionMgr) Publish(msg *packets.PublishPacket, session *Session) {

	v := this.publishDataPool.Get()
	if v != nil {
		d := v.(*mgrPublishData)
		d.msg = msg
		d.session = session

		this.publishList.Push(d)
		dashboard.Overview.RecvMsgCnt.Add(1)
		dashboard.Overview.CurPUblishBufferCnt.Set(int64(this.publishList.Len()))
	} else {
		log.Error("SessionMgr.Publish out of memory")
	}

}

func (this *SessionMgr) OnConnected(session *Session) {
	//不能阻塞session，不然会死锁
	this.connectedList.Push(session)

}

func (this *SessionMgr) OnDisconnected(session *Session) {
	//不能阻塞session，不然会死锁
	this.disconnectList.Push(session)
}

//连接验证超时
func (this *SessionMgr) OnConnectTimeout(session *Session) {
	//不能阻塞session，不然会死锁
	this.connectTimeoutList.Push(session)
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
