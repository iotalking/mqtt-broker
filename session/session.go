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
	pt      *PacketAndToken

	//已经重发的次数
	retryCnt int
}

const (
	//已经通知sessionMgr关闭session
	closing = 1
	//session的Close执行完成
	closed = 2
)

type DisconnectReason int32

func (dr DisconnectReason) Error() string {
	switch dr {
	case 0:
		return "DISERR_OK"
	case 1:
		return "DISERR_SENT"
	case 2:
		return "DISERR_RECV"
	}
	return "DISERR_REASON_UNKNOWN"
}

const (
	//没有异常断开
	DISERR_OK DisconnectReason = 0
	//发送异常
	DISERR_SENT DisconnectReason = 1
	//接收异常
	DISERR_RECV DisconnectReason = 2
	//PING超时
	DISERR_PING_TIMEOUT DisconnectReason = 3
)

//通道要保存服务端mqtt会话数据，比如tipic过滤器等
type Session struct {
	mgr      SessionMgr
	clientId string
	channel  *Channel
	isServer bool
	runtine  *runtine.SafeRuntine

	//>0不能再发送消息和接入到消息
	clostep int32
	//是否已经连接成功
	//作为服务端时，已经接入到CONNECT包，并验证通过
	//作为客户端时，表示服务端已经通过CONNECT包的验证
	//==0:验证超时
	//<0:表示验证失败的原因返回码的负值
	//>0:验证成功
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
	//从客户端的CONNECT命令中获取的keepalive
	keepalive int64 //nano 纳秒

	//发送中的消息，超时需要重发
	//带MessageID的消息才会插入inflightList
	inflightingList *utils.List

	peddingMsgList *utils.List
	//用来控制peddingMsgList的最大缓冲大小
	peddingChan chan byte

	onMessageCb      func(*packets.PublishPacket)
	onDisconnectedCb func()
	mtxSet           sync.Mutex

	connectToken    *ConnectToken
	disconnectToken *DisconnectToken

	willFlag    bool
	willRetain  bool
	willTopic   string
	willQos     byte
	willMessage []byte

	//是否已经异常断开
	hadDisconnectException int32
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
func NewSession(mgr SessionMgr, conn io.ReadWriteCloser, isServer bool) *Session {
	s := &Session{
		mgr:             mgr,
		isServer:        isServer,
		keepalive:       config.PingTimeout,
		publishChan:     make(chan *packets.PublishPacket),
		remoteMsgChan:   make(chan packets.ControlPacket),
		sentMsgChan:     make(SentChan, 1),
		inflightingList: utils.NewList(),
		peddingMsgList:  utils.NewList(),
		peddingChan:     make(chan byte, config.MaxSizeOfPublishMsg),
	}
	NewChannel(conn, s)

	return s
}

func (this *Session) SetClientId(id string) {
	this.clientId = id
}
func (this *Session) SetOnMessage(cb func(*packets.PublishPacket)) {
	this.mtxSet.Lock()
	defer this.mtxSet.Unlock()
	this.onMessageCb = cb
}

func (this *Session) SetOnDisconnected(cb func()) {
	this.mtxSet.Lock()
	defer this.mtxSet.Unlock()
	this.onDisconnectedCb = cb
}

func (this *Session) callbackOnDisconnected() {
	this.mtxSet.Lock()
	cb := this.onDisconnectedCb
	this.mtxSet.Unlock()

	if cb != nil {
		cb()
	}
}
func (this *Session) callbackOnMessage(msg *packets.PublishPacket) {
	this.mtxSet.Lock()
	onMsg := this.onMessageCb
	this.mtxSet.Unlock()

	if onMsg != nil {
		onMsg(msg)
	}
}

func (this *Session) insert2Inflight(pt *PacketAndToken) (err error) {
	if this.channel.IsStop() {
		return errors.New("%s channel is stoped")
	}
	var msgtype int

	switch pt.p.(type) {
	case *packets.PublishPacket:
		msgtype = packets.Publish

	case *packets.PubrecPacket:
		msgtype = packets.Pubrec
	case *packets.PubrelPacket:
		msgtype = packets.Pubrel

	}
	if msgtype > 0 {
		imsg := &inflightingMsg{
			pt:      pt,
			timeout: time.Now().UnixNano() + config.SentTimeout,
		}
		this.inflightingList.Push(imsg)
	}
	this.channel.Send(pt.p)
	return nil
}
func (this *Session) Send(msg packets.ControlPacket) (token Token, err error) {
	if this.channel.IsStop() {
		return nil, errors.New("%s channel is stoped")
	}
	pt := &PacketAndToken{
		p: msg,
		t: newToken(msg.Type()),
	}
	token = pt.t
	var msgtype int

	switch msg.(type) {
	case *packets.ConnectPacket:
		this.connectToken = token.(*ConnectToken)
	case *packets.DisconnectPacket:
		this.disconnectToken = token.(*DisconnectToken)
	case *packets.PublishPacket:
		msgtype = packets.Publish
		pubmsg := msg.(*packets.PublishPacket)
		pubmsg.MessageID = uint16(atomic.AddInt64(&this.lastMsgId, 1))
	case *packets.PubrecPacket:
		msgtype = packets.Pubrec
	case *packets.PubrelPacket:
		msgtype = packets.Pubrel
	case *packets.SubscribePacket:
		msgtype = packets.Subscribe
		subMsg := msg.(*packets.SubscribePacket)
		subToken := token.(*SubscribeToken)
		subToken.subs = subMsg.Topics
		subMsg.MessageID = uint16(atomic.AddInt64(&this.lastMsgId, 1))
	case *packets.UnsubscribePacket:
		unsubMsg := msg.(*packets.UnsubscribePacket)
		unsubMsg.MessageID = uint16(atomic.AddInt64(&this.lastMsgId, 1))
		msgtype = packets.Unsubscribe
	}
	if msgtype > 0 {
		imsg := &inflightingMsg{
			pt:      pt,
			timeout: time.Now().UnixNano() + config.SentTimeout,
		}
		this.inflightingList.Push(imsg)
	}
	return token, this.channel.Send(msg)
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
	log.Debug("session channel.Close returned")
	if this.isServer == false {
		if this.IsConnected() {

			if this.disconnectToken != nil {
				log.Debug("disconnectToken will flowComplete")
				this.disconnectToken.flowComplete()
			}
		} else {
			//还没有验证成功就被Close了，那一定是验证超时,connected为负数不验证失败，
			//验证失败在onConnack里处理
			if atomic.LoadInt32(&this.connected) == 0 {
				//验证超时
				if this.connectToken != nil {
					this.connectToken.err = errors.New("timeout")
					this.connectToken.flowComplete()
				}
			}

		}

	}
	this.callbackOnDisconnected()
	log.Debug("Session.Close will return")
	return
}

//发送ping消息
func (this *Session) Ping() {
	if !this.IsConnected() {
		log.Debug("Session is not connected")
		return
	}
	pingMsg := packets.NewControlPacket(packets.Pingreq)
	this.channel.Send(pingMsg)
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

	log.Debug("procFrontRemoteMsg msg :", msg.String())

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
func (this *Session) Publish(msg *packets.PublishPacket) (token Token, err error) {
	defer func() {
		//this.peddingChan 关闭后会panic,不关闭会死锁
		recover()
	}()
	msg.MessageID = uint16(atomic.AddInt64(&this.lastMsgId, 1))
	pt := &PacketAndToken{
		p: msg,
		t: newToken(msg.Type()),
	}
	token = pt.t
	if !this.IsConnected() || this.IsClosed() {
		err = errors.New("Publish session is stoped")
		return
	}
	if this.inflightingList.Len() < config.MaxSizeOfInflight &&
		this.channel.iSendList.Len() < config.MaxSizeOfPublishMsg {
		this.insert2Inflight(pt)
	} else {
		this.peddingChan <- 1
		this.peddingMsgList.Push(pt)
	}
	return
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
		if this.isServer {
			//接入PINGREQ超时，直接断开连接
			if this.IsConnected() {
				this.mgrOnPingTimeout()
			} else {
				this.mgrOnConnectTimeout()
			}

		} else {
			//客户端主动发送心跳包
			this.Ping()
			log.Debug("ping")
		}

	}
	//检测有没有要生发的消息
	this.checkInflightList()
	if this.isServer {
		this.mgr.BroadcastSessionInfo(this)
	}

}

func (this *Session) mgrOnDisconnected() {
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}
	this.mgr.GetSubscriptionMgr().RemoveSession(this)
	atomic.StoreInt32(&this.clostep, closing)
	this.mgr.OnDisconnected(this)
}

func (this *Session) mgrOnPingTimeout() {
	log.Info("mgrOnPingTimeout")
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}
	this.mgr.GetSubscriptionMgr().RemoveSession(this)
	atomic.StoreInt32(&this.clostep, closing)
	this.mgr.OnPingTimeout(this)
	go this.onDisconnectException(DISERR_PING_TIMEOUT)
}
func (this *Session) mgrOnConnectTimeout() {
	if atomic.LoadInt32(&this.clostep) > 0 {
		return
	}
	this.mgr.GetSubscriptionMgr().RemoveSession(this)
	atomic.StoreInt32(&this.clostep, closing)
	this.mgr.OnConnectTimeout(this)
}

func (this *Session) updateInflightMsg(msg packets.ControlPacket) (imsg *inflightingMsg) {

	this.inflightingList.Each(func(v interface{}) (stop bool) {
		imsg = v.(*inflightingMsg)
		if imsg.pt.p.Details().MessageID == msg.Details().MessageID {
			imsg.pt.p = msg
		}
		return
	})
	return
}

func (this *Session) removeInflightMsg(msgId uint16) (imsg *inflightingMsg) {

	this.inflightingList.Remove(func(v interface{}) (del, c bool) {
		imsg = v.(*inflightingMsg)
		if imsg.pt.p.Details().MessageID == msgId {
			del = true
			c = false
			return
		}
		del = false
		c = true
		imsg = nil
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
			this.channel.Send(imsg.pt.p)
		}
	}

	if this.inflightingList.Len() < config.MaxSizeOfInflight &&
		this.channel.iSendList.Len() < config.MaxSizeOfPublishMsg {
		select {
		case <-this.peddingChan:
		default:
		}
		v := this.peddingMsgList.Pop()
		if v != nil {
			this.insert2Inflight(v.(*PacketAndToken))
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
		this.resetTimeout()
	} else {
		go this.onDisconnectException(DISERR_RECV)
	}
}

func (this *Session) resetTimeout() {
	//服务端判断ping有没有超时
	if this.isServer {
		if this.IsConnected() {
			atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+this.keepalive)
		} else {
			atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+config.ConnectTimeout)
		}

	} else {
		//客户端判断接收PINGRESP超时
		atomic.StoreInt64(&this.timeout, time.Now().UnixNano()+this.keepalive)
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
				this.onInflightDone(msg)
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
		this.resetTimeout()
	} else {
		//写异常
		go this.onDisconnectException(DISERR_SENT)
	}
}

//异常断开发送will消息
func (this *Session) onDisconnectException(reason DisconnectReason) {
	if DisconnectReason(atomic.LoadInt32(&this.hadDisconnectException)) == DISERR_OK {
		if this.willFlag {
			msg := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
			msg.TopicName = this.willTopic
			msg.Qos = this.willQos
			msg.Payload = this.willMessage
			this.sendToSubcriber(msg)
			//保存willRetain的消息
			if this.willRetain {
				err := this.mgr.GetStoreMgr().SaveRetainMsg(msg.TopicName, msg.Payload, msg.Qos)
				if err != nil {
					log.Error(err)
					return
				}
			}
		}
		atomic.StoreInt32(&this.hadDisconnectException, int32(reason))
	}
}
func (this *Session) SetKeepalive(keepalive int64) {
	this.keepalive = keepalive
}
func (this *Session) GetKeepalive() int64 {
	return this.keepalive
}
