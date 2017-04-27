package session

import (
	"net"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/config"
	"github.com/iotalking/mqtt-broker/safe-runtine"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/iotalking/mqtt-broker/ticker"
	"github.com/iotalking/mqtt-broker/utils"
)

//通道要保存服务端mqtt会话数据，比如tipic过滤器等
type Session struct {
	mgr      *SessionMgr
	clientId string
	channel  *Channel
	isServer bool
	runtine  *runtine.SafeRuntine

	//>0表示关闭，不能再发送消息和接入到消息
	closed int32
	//是否已经连接成功
	//作为服务端时，已经接入到CONNECT包，并验证通过
	//作为客户端时，表示服务端已经通过CONNECT包的验证
	connected int32

	//runtine的启动时间戳
	startTime time.Time

	//连接超时定时器
	connectTmTimer *ticker.Timer
	connectTmChan  chan struct{}
	//客户端连接数据
	connectResultChan chan error

	//最新的消息ID
	//递增1
	lastMsgId int64

	//重发时间到时告诉sesssion重发sendingMap中的某个消息
	resendChan chan interface{}

	//Publish消息的chan
	publishList *utils.List

	//从远端接收到的消息队列
	remoteMsgList *utils.List

	//心跳包定时器
	pingTimer *ticker.Timer
	//心跳应答包超时定时器
	//接收到消息和已经发送消息都要重置此定时器
	pingrespTimer *ticker.Timer

	//Channel发送完一个消息
	//这里注意不能用阻塞发送，可能会导致session和channel.sendRun死锁
	sentMsgChan SentChan
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
func NewSession(mgr *SessionMgr, conn net.Conn, isServer bool) *Session {
	s := &Session{
		mgr:           mgr,
		isServer:      isServer,
		resendChan:    make(chan interface{}),
		publishList:   utils.NewList(),
		remoteMsgList: utils.NewList(),
		sentMsgChan:   make(SentChan, 1),

		pingTimer:      ticker.NewTimer(nil),
		pingrespTimer:  ticker.NewTimer(nil),
		connectTmTimer: ticker.NewTimer(nil),
	}
	s.channel = NewChannel(conn, s)

	runtine.Go(func(r *runtine.SafeRuntine) {
		s.runtine = r
		s.startTime = time.Now()
		log.Debug("session running")
		s.run()
	})
	return s
}

func (this *Session) run() {
	//先启动connect检测定时器
	//作为服务端时，如果超时，直接将连接断开，未超时，则要取消此定时器
	//作为客户端时，如果超时，直接断开连接，将告诉调用者连接失败
	this.connectTmTimer.Reset(time.Duration(config.ConnectTimeout)*time.Second, nil)
	log.Debug("session.run")

	defer func() {
		this.pingrespTimer.Stop()
		this.pingTimer.Stop()
		this.connectTmTimer.Stop()

		atomic.StoreInt32(&this.connected, 0)
		atomic.StoreInt32(&this.closed, 1)

		log.Debug("session exit")

	}()

	var err error
	for {
		log.Debug("session select [[")
		select {
		case err := <-this.channel.Error():
			//channel有错误发生
			log.Error("channel err:", err)
			if this.IsConnected() {
				log.Debug("session.mgr.OnDisconnected")
				this.mgr.OnDisconnected(this)
			} else {
				log.Debug("session.mgr.OnConnectTimeout")
				this.mgr.OnConnectTimeout(this)
			}
		//接入客户的CONNECT消息超时
		case <-this.runtine.IsInterrupt:
			log.Debugf("Session run IsInterrupt")
			return

		case <-this.pingTimer.Wait():
			//要分服务端和客户端
			if this.isServer {
				//心跳包超时，断开连接
				this.mgr.OnDisconnected(this)
				break
			} else {
				this.Ping()
			}
		case <-this.pingrespTimer.Wait():
			//客户接入心跳应该包超时，断开连接
			this.mgr.OnDisconnected(this)
			break
		case <-this.connectTmTimer.Wait():
			this.mgr.OnConnectTimeout(this)
			break
		case data := <-this.resendChan:

			//要重发msgId
			if _sendingData, ok := data.(*sendingData); ok {
				if publishMsg, ok2 := _sendingData.msg.(*packets.PublishPacket); ok2 {
					publishMsg.Dup = true
				}
				this.Send(_sendingData.msg, nil)
				_sendingData.retryCount++
				_sendingData.retryTimer.Reset(time.Duration(config.SentTimeout)*time.Second, _sendingData)
			}

		case <-this.sentMsgChan:
			//channel发送完一个消息，会告诉session，以重置心跳包定时器
			log.Debug("session sentMsgChan recv")
			this.ResetPingTimer()
		case <-this.remoteMsgList.Wait():
			err = this.procFrontRemoteMsg()
			if err != nil {
				break
			}
		case <-this.publishList.Wait():
			//发送前面的消息
			this.publishFront()
		}
		log.Debug("session select ]]")
		if err != nil {
			//如果处理消息有错误发生，则断开连接
			log.Errorf("process input message error:", err)
			this.channel.Close()
			return
		}
	}
}

func (this *Session) Send(msg packets.ControlPacket, ch SentChan) {
	if ch == nil {
		log.Debug("session.Send:", msg, this.sentMsgChan)
		this.channel.Send(msg, this.sentMsgChan)
	} else {
		this.channel.Send(msg, ch)
	}

}

//判断是否已经连接成功
//看CONNECT消息有没有处理完成
//如果IsClosed返回true,那么IsConnected一定返回true
func (this *Session) IsConnected() bool {
	return atomic.LoadInt32(&this.connected) > 0
}

//判断是否已经关闭
func (this *Session) IsClosed() bool {
	return atomic.LoadInt32(&this.closed) > 0
}

//关闭
func (this *Session) Close() {
	if this.IsClosed() {
		log.Debug("session is closed")
		return
	}
	log.Debug("session closing channel")
	this.channel.Close()
	log.Debug("session stoping self runtine")
	this.runtine.Stop()
	log.Debug("session stop finish")

	return
}

func (this *Session) ResetPingTimer() {
	if this.IsClosed() {
		return
	}
	this.pingTimer.Stop()
	this.pingrespTimer.Stop()
}

//发送ping消息
func (this *Session) Ping() {
	if this.IsConnected() {
		return
	}
	pingMsg := &packets.PingreqPacket{}
	this.Send(pingMsg, nil)
}

//给服务器发送连接消息
//收到CONNACK或者超时时，通过SentChan告诉调用者，连接成功还是失败
func (this *Session) Connect(msg *packets.ConnectPacket) <-chan error {
	if this.IsConnected() {
		return nil
	}
	if this.isServer {
		//只有客户端才可以调用
		panic("server can't call this function")
		return nil
	}
	this.Send(msg, nil)
	this.connectResultChan = make(chan error)
	return this.connectResultChan
}

func (this *Session) RecvMsg(msg packets.ControlPacket) {
	this.remoteMsgList.Push(msg)
}
func (this *Session) procFrontRemoteMsg() (err error) {
	v := this.remoteMsgList.Pop()
	if v == nil {
		return
	}
	var msg = v.(packets.ControlPacket)
	log.Debugf("procFrontRemoteMsg msg :", msg.String())

	this.ResetPingTimer()
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
			this.mgr.OnDisconnected(this)
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
func (this *Session) Publish(msg *packets.PublishPacket) {
	this.publishList.Push(msg)
}

func (this *Session) publishFront() {
	v := this.publishList.Pop()
	if v != nil {
		d := v.(*packets.PublishPacket)
		this.onPublishData(d)
	}
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
		this.connectResultChan <- packets.ConnErrors[packets.ErrNetworkError]
	}
	atomic.StoreInt32(&this.closed, 1)
	atomic.StoreInt32(&this.connected, 0)
}
