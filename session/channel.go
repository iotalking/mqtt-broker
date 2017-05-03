package session

import (
	"errors"
	"net"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/iotalking/mqtt-broker/config"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/safe-runtine"
)

type SentChan chan *SendData

type SendData struct {
	Msg packets.ControlPacket

	Result chan<- error
}

type MsgReceiver interface {
	RecvMsg(packets.ControlPacket)
	OnChannelError(error)
}
type Channel struct {
	//>0为已经退出
	isStoped int32
	//下层数据通讯接口
	conn net.Conn

	//接入从调用者写入的消息
	sendChan chan packets.ControlPacket

	//写入从net.Conn里接入到的消息
	msgReceiver MsgReceiver
	//错误chan
	errChn      chan error
	recvRuntine *runtine.SafeRuntine
	sendRuntine *runtine.SafeRuntine

	//取后通讯的时间戳，即最后发包，收包时间
	lastStamp int64
}

//New 创建通道
//通过网络层接口进行数据通讯
func NewChannel(c net.Conn, msgReceiver MsgReceiver) *Channel {
	var channel = &Channel{
		conn:        c,
		sendChan:    make(chan packets.ControlPacket, config.MaxSizeOfSendChannel),
		msgReceiver: msgReceiver,
		errChn:      make(chan error),
	}
	channel.recvRuntine = runtine.Go(func(r *runtine.SafeRuntine, args ...interface{}) {
		channel.recvRuntine = r
		log.Debug("channel recv running")
		channel.recvRun()
	})
	channel.sendRuntine = runtine.Go(func(r *runtine.SafeRuntine, args ...interface{}) {
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

		if err == nil {
			this.msgReceiver.RecvMsg(msg)
			dashboard.Overview.RecvMsgCnt.Add(1)
			log.Debug("channel recv a msg:", msg.String())
		} else {
			this.msgReceiver.OnChannelError(err)
			log.Error("recvRun error:", err)
			break
		}

	}
	log.Debug("channel recvRun exited")
}

func (this *Channel) sendRun() {

	defer func() {
		atomic.StoreInt32(&this.isStoped, 1)
		close(this.sendChan)
		log.Error("Channel sendChan has closed")
	}()
	for {
		select {

		case <-this.sendRuntine.IsInterrupt:
			//要求安全退出
			log.Debugln("recvRun IsInterrupt has closed:")
			return
		case msg := <-this.sendChan:
			//处理上层的消息
			err := msg.Write(this)
			//如果写失败，则退出runtine
			if err != nil {
				log.Error("sendRun write msg to conn error", err)
				dashboard.Overview.SentErrClientCnt.Add(1)
				return
			}

			dashboard.Overview.SentMsgCnt.Add(1)
		}
	}
	log.Debug("channel recvRun exited")
}

//Send 将消息写入发送channel
//如果channel的buffer满后，会阻塞
func (this *Channel) Send(msg packets.ControlPacket) (err error) {
	if this.IsStop() {
		log.Debug("channel is closed")
		return
	}
	defer func() {
		if recover() != nil {
			err = errors.New("channel.sendChan is closed")
		}
	}()
	if publishMsg, ok := msg.(*packets.PublishPacket); ok {
		switch publishMsg.Details().Qos {
		case 0:
			this.sendChan <- msg
		case 1, 2:
			this.sendChan <- msg
		default:
			log.Debugf("Channel qos error drop msg")
		}
	} else {
		this.sendChan <- msg
	}
	return
}

//安全退出
func (this *Channel) Close() {
	if atomic.LoadInt32(&this.isStoped) > 0 {
		log.Debug("Channel has closed")
		return
	}
	//把下层的net.Conn关闭,让recvChn从net.Conn的接入中退出
	atomic.StoreInt32(&this.isStoped, 1)
	this.conn.Close()
	dashboard.Overview.OpenedFiles.Add(-1)
	this.sendRuntine.Stop()
	this.sendRuntine = nil
	this.recvRuntine.Stop()
	this.recvRuntine = nil
	this.conn = nil

}

func (this *Channel) IsStop() bool {
	return atomic.LoadInt32(&this.isStoped) > 0
}
