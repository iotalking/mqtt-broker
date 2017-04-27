package session

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/iotalking/mqtt-broker/config"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/safe-runtine"

	"github.com/iotalking/mqtt-broker/utils"
)

type SentChan chan *SendData

type SendData struct {
	Msg packets.ControlPacket
	//发送结果
	Err error
	//发送完成会把SendData发给调用者，以告诉其发送结果
	SChan SentChan
}

type MsgReceiver interface {
	RecvMsg(packets.ControlPacket)
}
type Channel struct {
	//>0为已经退出
	isStoped int32
	//下层数据通讯接口
	conn net.Conn

	//接入从调用者写入的消息
	sendList     *utils.List
	sendDataPool *sync.Pool

	//写入从net.Conn里接入到的消息
	msgReceiver MsgReceiver
	//错误chan
	errChn      chan error
	recvRuntine *runtine.SafeRuntine
	sendRuntine *runtine.SafeRuntine
}

//New 创建通道
//通过网络层接口进行数据通讯
func NewChannel(c net.Conn, msgReceiver MsgReceiver) *Channel {
	var channel = &Channel{
		conn:     c,
		sendList: utils.NewList(),
		sendDataPool: &sync.Pool{
			New: func() interface{} {
				return &SendData{}
			},
		},
		msgReceiver: msgReceiver,
		errChn:      make(chan error),
	}
	channel.recvRuntine = runtine.Go(func(r *runtine.SafeRuntine) {
		channel.recvRuntine = r
		log.Debug("channel recv running")
		channel.recvRun()
	})
	channel.sendRuntine = runtine.Go(func(r *runtine.SafeRuntine) {
		channel.sendRuntine = r
		log.Debug("channel send running")
		channel.sendRun()
	})
	return channel
}
func (this *Channel) Read(p []byte) (n int, err error) {
	n, err = this.conn.Read(p)
	dashboard.Overview.RecvBytes.Add(int64(n))
	return
}
func (this *Channel) Write(p []byte) (n int, err error) {
	n, err = this.conn.Write(p)
	dashboard.Overview.SentBytes.Add(int64(n))
	return
}

//从底层接入消息循环
//从net.Conn里流式解包消息
func (this *Channel) recvRun() {
	for {

		msg, err := packets.ReadPacket(this)

		log.Debugln("recvRun error:", err)
		if err != nil {
			select {
			case this.errChn <- err:
			default:
			}
			break
		} else {
			this.msgReceiver.RecvMsg(msg)
			log.Debug("channel recv a msg:", msg.String())

		}

	}
	log.Debug("channel recvRun exited")
}

func (this *Channel) sendRun() {

	for {
		select {

		case <-this.sendRuntine.IsInterrupt:
			//要求安全退出
			log.Debugln("recvRun IsInterrupt has closed:")
			return
		case <-this.sendList.Wait():
			//发送最前面的消息
			this.sendFrontMsg()
		}
	}
	log.Debug("channel recvRun exited")
}

//内部使用
func (this *Channel) sendFrontMsg() {
	v := this.sendList.Pop()
	if v != nil {
		//data用完后要还给pool
		data := v.(*SendData)
		//设置写超时
		this.conn.SetWriteDeadline(time.Now().Add(time.Duration(config.SentTimeout) * time.Second))
		//处理上层的消息
		err := data.Msg.Write(this)
		this.sendDataPool.Put(data)

		if data.SChan != nil {
			data.Err = err
			//调用发送回调函数
			select {
			case data.SChan <- data:
			default:
			}

		}
		log.Debug("channel send msg :", data)
		//如果写失败，则退出runtine
		if err != nil {
			log.Debugln("sendRun write msg to conn error", err)
			dashboard.Overview.SentErrClientCnt.Add(1)
			select {
			case this.errChn <- err:
			default:
			}
			return
		}

		dashboard.Overview.SentMsgCnt.Add(1)
	}
}

//Send 将消息写入发送channel
//如果channel的buffer满后，会阻塞
func (this *Channel) Send(msg packets.ControlPacket, sc SentChan) {
	_sendData := &SendData{
		Msg:   msg,
		SChan: sc,
	}
	if publishMsg, ok := msg.(*packets.PublishPacket); ok {
		switch publishMsg.Details().Qos {
		case 0:
			this.sendList.Push(_sendData)
		case 1, 2:
			this.sendList.Push(_sendData)
		default:
			log.Debugf("Channel qos error drop msg")
		}
	} else {
		this.sendList.Push(_sendData)
	}

}

func (this *Channel) Error() <-chan error {
	return this.errChn
}

//安全退出
func (this *Channel) Close() {
	if atomic.LoadInt32(&this.isStoped) > 0 {
		return
	}
	//把下层的net.Conn关闭,让recvChn从net.Conn的接入中退出
	this.conn.Close()

	this.sendRuntine.Stop()
	this.sendRuntine = nil
	this.recvRuntine.Stop()
	this.recvRuntine = nil
	this.conn = nil

	atomic.StoreInt32(&this.isStoped, 1)
}
