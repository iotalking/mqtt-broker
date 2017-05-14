package session

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/safe-runtine"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/iotalking/mqtt-broker/config"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/ticker"
	"github.com/iotalking/mqtt-broker/utils"
)

type inflightingMsg struct {
	//超时时间,纳秒
	timeout int64
	msg     packets.ControlPacket

	msgType int
	//已经重发的次数
	retryCnt int
}

const (
	//已经通知sessionMgr关闭session
	closing = 1
	//session的Close执行完成
	closed = 2
)

//通道要保存服务端mqtt会话数据，比如tipic过滤器等
type Session struct {
	mgr      *SessionMgr
	clientId string
	channel  *Channel
	isServer bool
	runtine  *runtine.SafeRuntine

	//>0不能再发送消息和接入到消息
	clostep int32
	//是否已经连接成功
	//作为服务端时，已经接入到CONNECT包，并验证通过
	//作为客户端时，表示服务端已经通过CONNECT包的验证
	connected int32

	//runtine的启动时间戳
	startTime time.Time

	//最新的消息ID
	//递增1
	lastMsgId int64

	//Publish消息的chan
	publishChan chan *packets.PublishPacket
	//从远端接收到的消息队列
	remoteMsgChan chan packets.ControlPacket

	//Channel发送完一个消息
	//这里注意不能用阻塞发送，可能会导致session和channel.sendRun死锁
	sentMsgChan SentChan

	//最后一次读写成功的时间
	timeout int64

	//发送中的消息，超时需要重发
	//TODO 用msgId做为Map存储inflighting消息，可以优先内存以避免反复分配
	inflightingList *utils.List

	peddingMsgList *utils.List
	//用来控制peddingMsgList的最大缓冲大小
	peddingChan chan byte

	onMessage func(*packets.PublishPacket)
	mtxOnMsg  sync.Mutex
}

//使用Session的一方调用Publish函数时，通过publishChan给Session传消息用
type publishData struct {
	msg        *packets.PublishPacket
	resultChan chan error
}

//保存已经发到网络的消息想着数据
type sendingData struct {
	//发送中的消息
	msg packets.ControlPacket
	//接入发送结果的chan
	resultChan chan error
	//重发定时器
	//如果timer不为空，则到收到应答后要取消timer
	retryTimer *ticker.Timer

	//已经重试的次数
	retryCount int
}

//创建会话
//客户端会话和服务端会议的主要区别只是要不要发ping消息
func NewSession(mgr *SessionMgr, conn io.ReadWriteCloser, isServer bool) *Session {
	s := &Session{
		mgr:             mgr,
		isServer:        isServer,
		publishChan:     make(chan *packets.PublishPacket),
		remoteMsgChan:   make(chan packets.ControlPacket),
		sentMsgChan:     make(SentChan, 1),
		inflightingList: utils.NewList(),
		peddingMsgList:  utils.NewList(),
		peddingChan:     make(chan byte, config.MaxSizeOfPublishMsg),
	}
	s.channel = NewChannel(conn, s)

	return s
}

func (this *Session) RegisterOnMessage(cb func(*packets.PublishPacket)) {
	this.mtxOnMsg.Lock()
	defer this.mtxOnMsg.Unlock()
	this.onMessage = cb
}
func (this *Session) callbackOnMessage(msg *packets.PublishPacket) {
	this.mtxOnMsg.Lock()
	onMsg := this.onMessage
	this.mtxOnMsg.Unlock()

	if onMsg != nil {
		onMsg(msg)
	}
}

func (this *Session) insert2Inflight(msg packets.ControlPacket) (err error) {
	if this.channel.IsStop() {
		return errors.New("%s channel is stoped")
	}
	var msgtype int

	switch msg.(type) {
	case *packets.PublishPacket:
		if msg.Details().Qos > 0 {
			msgtype = packets.Publish
		}

	case *packets.PubrecPacket:
		msgtype = packets.Pubrec
	case *packets.PubrelPacket:
		msgtype = packets.Pubrel

	}
	if msgtype > 0 {
		imsg := &inflightingMsg{
			msg:     msg,
			msgType: msgtype,
			timeout: time.Now().UnixNano() + config.SentTimeout,
		}
		this.inflightingList.Push(imsg)
	}
	this.channel.Send(msg)
	return nil
}
func (this *Session) Send(msg packets.ControlPacket) (err error) {
	if this.channel.IsStop() {
		return errors.New("%s channel is stoped")
	}
	var msgtype int

	switch msg.(type) {
	case *packets.PublishPacket:
		if msg.Details().Qos > 0 {
			msgtype = packets.Publish
		}
		pubmsg := msg.(*packets.PublishPacket)
		pubmsg.MessageID = uint16(atomic.AddInt64(&this.lastMsgId, 1))
	case *packets.PubrecPacket:
		msgtype = packets.Pubrec
	case *packets.PubrelPacket:
		msgtype = packets.Pubrel

	}
	if msgtype > 0 {
		imsg := &inflightingMsg{
			msg:     msg,
			msgType: msgtype,
			timeout: time.Now().UnixNano() + config.SentTimeout,
		}
		this.inflightingList.Push(imsg)
	}
	return this.channel.Send(msg)
}

//判断是否已经连接成功
//看CONNECT消息有没有处理完成
//如果IsClosed返回true,那么IsConnected一定返回true
func (this *Session) IsConnected() bool {
	return atomic.LoadInt32(&this.connected) > 0
}

//判断是否已经关闭
func (this *Session) IsClosed() bool {
	return atomic.LoadInt32(&this.clostep) == closed
}

//

//关闭
func (this *Session) Close() {
	if atomic.LoadInt32(&this.clostep) == closed {
		log.Debug("session is clostep")
		return
	}
	atomic.StoreInt32(&this.clostep, closed)
	log.Debug("session closing channel")
	close(this.peddingChan)
	this.channel.Close()
	log.Debug("session is clostep")
	return
}

//发送ping消息
func (this *Session) Ping() {
	if this.IsConnected() {
		return
	}
	pingMsg := &packets.PingreqPacket{}
	this.Send(pingMsg)
}

//给服务器发送连接消息
//收到CONNACK或者超时时，通过SentChan告诉调用者，连接成功还是失败
func (this *Session) Connect(msg *packets.ConnectPacket) error {
	if this.IsConnected() {
		return nil
	}
	if this.isServer {
		//只有客户端才可以调用
		panic("server can't call this function")
		return nil
	}
	return this.Send(msg)
}

func (this *Session) RecvMsg(msg packets.ControlPacket) {
	if atomic.LoadInt32(&this.clostep) > 0 {
		log.Debug("RecvMsg session is cloed")
		return
	}
	this.procFrontRemoteMsg(msg)
}

//处理从远端接收到的消息
func (this *Session) procFrontRemoteMsg(msg packets.ControlPacket) (err error) {

	log.Debugf("procFrontRemoteMsg msg :", msg.String())

	switch msg.(type) {
	case *packets.ConnectPacket:
		err = this.onConnect(msg.(*packets.ConnectPacket))
	case *packets.ConnackPacket:
		err = this.onConnack(msg.(*packets.ConnackPacket))
	case *packets.PublishPacket:
		err = this.onPublish(msg.(*packets.PublishPacket))
	case *packets.PubackPacket:
		err = this.onPuback(msg.(*packets.PubackPacket))
	case *packets.PubrecPacket:
		err = this.onPubrec(msg.(*packets.PubrecPacket))
	case *packets.PubrelPacket:
		err = this.onPubrel(msg.(*packets.PubrelPacket))
	case *packets.PubcompPacket:
		err = this.onPubcomp(msg.(*packets.PubcompPacket))
	case *packets.SubscribePacket:
		err = this.onSubscribe(msg.(*packets.SubscribePacket))
	case *packets.SubackPacket:
		err = this.onSuback(msg.(*packets.SubackPacket))
	case *packets.UnsubscribePacket:
		err = this.onUnsubscribe(msg.(*packets.UnsubscribePacket))
	case *packets.UnsubackPacket:
		err = this.onUnsuback(msg.(*packets.UnsubackPacket))
	case *packets.PingreqPacket:
		err = this.onPingreq(msg.(*packets.PingreqPacket))
	case *packets.PingrespPacket:
		err = this.onPingresp(msg.(*packets.PingrespPacket))
	case *packets.DisconnectPacket, nil:
		if msg != nil {
			err = this.onDisconnect(msg.(*packets.DisconnectPacket))
		} else {
			//如果channel的远程断开，则channel发一个nil给session以示退出
			//告诉sessionMgr关闭本session
			this.mgrOnDisconnected()
		}

	}
	return
}

//外部让session发布消息
//函数返回时，表示session runtine已经在处理了
//最终处理结果通过chan来接收，可以忽略结果
//qos=0时，session发送完数据就有结果
//qos=1时，session收到PUBACK后才有结果
//qos=2时, session收到PUBCOMP后才有结果
func (this *Session) Publish(msg *packets.PublishPacket) error {
	defer func() {
		//this.peddingChan 关闭后会panic,不关闭会死锁
		recover()
	}()
	if !this.IsConnected() || this.IsClosed() {
		return errors.New("Publish session is stoped")
	}
	if this.inflightingList.Len() < config.MaxSizeOfInflight &&
		this.channel.iSendList.Len() < config.MaxSizeOfPublishMsg {
		this.insert2Inflight(msg)
	} else {
		this.peddingChan <- 1
		this.peddingMsgList.Push(msg)
	}
	return nil
}

//连接消息超时
func (this *Session) onConnectTimeout() {
	log.Debug("Session onConnectTimeout")
	if this.IsConnected() {
		//已经连接不用理会
		return
	}
	if !this.isServer {
		//作为客户端时
	}
	atomic.StoreInt32(&this.clostep, 1)
	atomic.StoreInt32(&this.connected, 0)
}

func (this *Session) OnChannelError(err error) {
	log.Error("OnChannelError:", err)
	if this.IsConnected() {
		log.Debug("OnChannelError.mgr.OnDisconnected")
		this.mgrOnDisconnected()
	} else {
		log.Debug("OnChannelError.mgr.OnConnectTimeout")
		this.mgrOnConnectTimeout()
	}

}

//定时被调用，用于检查各种超时
func (this *Session) OnTick() {
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}
	now := time.Now().UnixNano()

	//服务端判断ping有没有超时
	if now >= atomic.LoadInt64(&this.timeout) {

		//接入PINGREQ超时，直接断开连接
		if this.IsConnected() {
			this.mgrOnDisconnected()
		} else {
			this.mgrOnConnectTimeout()
		}
		log.Debug("ping timeout")
	}
	//检测有没有要生发的消息
	this.checkInflightList()

	this.broadcastSessionInfo()
}

func (this *Session) mgrOnDisconnected() {
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}
	this.mgr.subscriptionMgr.RemoveSession(this)
	atomic.StoreInt32(&this.clostep, closing)
	this.mgr.OnDisconnected(this)
}
func (this *Session) mgrOnConnectTimeout() {
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}
	this.mgr.subscriptionMgr.RemoveSession(this)
	atomic.StoreInt32(&this.clostep, closing)
	this.mgr.OnConnectTimeout(this)
}

func (this *Session) updateInflightMsg(msg packets.ControlPacket) (imsg *inflightingMsg) {

	this.inflightingList.Each(func(v interface{}) (stop bool) {
		imsg = v.(*inflightingMsg)
		if imsg.msg.Details().MessageID == msg.Details().MessageID {
			imsg.msg = msg
		}
		return
	})
	return
}

func (this *Session) removeInflightMsg(msgId uint16) (imsg *inflightingMsg) {

	this.inflightingList.Remove(func(v interface{}) (del, c bool) {
		imsg = v.(*inflightingMsg)
		if imsg.msg.Details().MessageID == msgId {
			del = true
			c = true
			return
		}
		del = false
		c = true
		return
	})
	return
}

//检测重发队列是否有超时的消息
func (this *Session) checkInflightList() {
	if this.inflightingList.Len() > 0 {

		now := time.Now().UnixNano()

		var tmMsg = make([]*inflightingMsg, 0, this.inflightingList.Len())

		this.inflightingList.Each(func(v interface{}) (stop bool) {
			imsg := v.(*inflightingMsg)
			if imsg.timeout <= now {
				tmMsg = append(tmMsg, imsg)
			}
			return
		})

		if len(tmMsg) > 0 {
			log.Debugf("Session[%s] has msg to resend", this.clientId)
		}
		for _, imsg := range tmMsg {
			//超时，需要重发
			imsg.retryCnt++
			imsg.timeout = now + config.SentTimeout
			this.channel.Send(imsg.msg)
		}
	}

	if this.inflightingList.Len() < config.MaxSizeOfInflight {
		select {
		case <-this.peddingChan:
		default:
		}
		v := this.peddingMsgList.Pop()
		if v != nil {
			this.insert2Inflight(v.(packets.ControlPacket))
		}
	}
}

//channel调用conn读返回后的回调函数
//可能是timeout
//此函数不能阻塞，要尽快返回
func (this *Session) onChannelReaded(msg packets.ControlPacket, err error) {
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}
	if err == nil {
		switch msg.(type) {
		case *packets.ConnectPacket:
			dashboard.Overview.ConnectRecvCnt.Add(1)
		case *packets.ConnackPacket:
			dashboard.Overview.ConnackRecvCnt.Add(1)
		case *packets.PublishPacket:
			dashboard.Overview.PublishRecvCnt.Add(1)
		case *packets.PubackPacket:
			dashboard.Overview.PubackRecvCnt.Add(1)
		case *packets.PubrecPacket:
			dashboard.Overview.PubrecRecvCnt.Add(1)
		case *packets.PubrelPacket:
			dashboard.Overview.PubrelRecvCnt.Add(1)
		case *packets.PubcompPacket:
			dashboard.Overview.PubcompRecvCnt.Add(1)
		case *packets.SubscribePacket:
			dashboard.Overview.SubscribeRecvCnt.Add(1)
		case *packets.SubackPacket:
			dashboard.Overview.SubackRecvCnt.Add(1)
		case *packets.UnsubscribePacket:
			dashboard.Overview.UnsubscribeRecvCnt.Add(1)
		case *packets.UnsubackPacket:
			dashboard.Overview.UnsubackRecvCnt.Add(1)
		case *packets.PingreqPacket:
			dashboard.Overview.PingreqRecvCnt.Add(1)
		case *packets.PingrespPacket:
			dashboard.Overview.PingrespRecvCnt.Add(1)
		case *packets.DisconnectPacket:
			dashboard.Overview.DisconectRecvCnt.Add(1)
		}
		//服务端判断ping有没有超时
		if this.isServer {
			if this.IsConnected() {
				atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+config.PingTimeout)
			} else {
				atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+config.ConnectTimeout)
			}

		} else {
			//客户端判断接收PINGRESP超时
			atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+config.PingrespTimeout)
		}

	}

}

//channel调用conn写返回后的回调函数
//可能是timeout
//不能在此函数里调用channel.Send,否则会死锁
//需要重发包可以通过调用channel.Resend
func (this *Session) onChannelWrited(msg packets.ControlPacket, err error) {
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}

	if err == nil {
		switch msg.(type) {
		case *packets.ConnectPacket:
			dashboard.Overview.ConnectSentCnt.Add(1)
		case *packets.ConnackPacket:
			dashboard.Overview.ConnackSentCnt.Add(1)
		case *packets.PublishPacket:
			if msg.Details().Qos == 0 {
				dashboard.Overview.SentMsgCnt.Add(1)
			}
			dashboard.Overview.PublishSentCnt.Add(1)
		case *packets.PubackPacket:
			dashboard.Overview.PubackSentCnt.Add(1)
		case *packets.PubrecPacket:
			dashboard.Overview.PubrecSentCnt.Add(1)
		case *packets.PubrelPacket:
			dashboard.Overview.PubrelSentCnt.Add(1)
		case *packets.PubcompPacket:
			dashboard.Overview.PubcompSentCnt.Add(1)
		case *packets.SubscribePacket:
			dashboard.Overview.SubscribeSentCnt.Add(1)
		case *packets.SubackPacket:
			dashboard.Overview.SubackSentCnt.Add(1)
		case *packets.UnsubscribePacket:
			dashboard.Overview.UnsubscribeSentCnt.Add(1)
		case *packets.UnsubackPacket:
			dashboard.Overview.UnsubackSentCnt.Add(1)
		case *packets.PingreqPacket:
			dashboard.Overview.PingreqSentCnt.Add(1)
		case *packets.PingrespPacket:
			dashboard.Overview.PingrespSentCnt.Add(1)
		case *packets.DisconnectPacket:
			dashboard.Overview.DisconectSentCnt.Add(1)
		}
		//服务端判断ping有没有超时
		if this.isServer {
			if this.IsConnected() {
				atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+config.PingTimeout)
			} else {
				atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+config.ConnectTimeout)
			}

		} else {
			//客户端判断接收PINGRESP超时
			atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+config.PingrespTimeout)
		}

	}
}
