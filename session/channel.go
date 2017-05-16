package session

import (
	"io"
	"net"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/safe-runtine"
	"github.com/iotalking/mqtt-broker/utils"
)

type SentChan chan *SendData

type SendData struct {
	Msg packets.ControlPacket

	Result chan<- error
}

type Channel struct {
	//>0为已经退出
	isStoped int32
	//下层数据通讯接口
	conn io.ReadWriteCloser

	//内部发送的消息队列,所有还在流程中的消息

	iSendList *utils.List
	//写入从net.Conn里接入到的消息
	session     *Session
	recvRuntine *runtine.SafeRuntine
	sendRuntine *runtine.SafeRuntine

	//取后通讯的时间戳，即最后发包，收包时间
	lastStamp int64
}

//New 创建通道
//通过网络层接口进行数据通讯
func NewChannel(c io.ReadWriteCloser, session *Session) *Channel {
	if c == nil {
		panic("NewChannel c == nil")
	}
	var channel = &Channel{
		conn:      c,
		session:   session,
		iSendList: utils.NewList(),
	}
	session.channel = channel
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

		//回调session，可用于检查有没有connect连时,或者ping超时
		this.session.onChannelReaded(msg, err)
		if err == nil {
			this.session.RecvMsg(msg)

			log.Debug("channel recv a msg:", msg.String())
		} else {
			this.session.OnChannelError(err)
			log.Error("recvRun error:", err)
			break
		}

	}
	log.Debug("channel recvRun exited")
}

func (this *Channel) sendRun() {

	defer func() {
		log.Debug("Channel.sendRun exited")
		atomic.StoreInt32(&this.isStoped, 1)
	}()
	for !this.sendRuntine.IsStoped() {
		select {
		case <-this.sendRuntine.IsInterrupt:
		case <-this.iSendList.Wait():
			for !this.sendRuntine.IsStoped() {
				v := this.iSendList.Pop()
				if v == nil {
					break
				}
				err := this.writeMsg(v.(packets.ControlPacket))
				if err != nil {
					log.Errorf("channel.writeMsg error:%s,msg:%#v", err, v)
					return
				}
			}
			this.session.checkInflightList()
		}
	}

}
func (this *Channel) writeMsg(msg packets.ControlPacket) (err error) {
	//处理上层的消息
	err = msg.Write(this)
	//回调session,用于检查有没有要重发的消息，ping超时等
	this.session.onChannelWrited(msg, err)
	//如果写失败，则退出runtine
	if err != nil {
		if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			log.Debug("channel write timeout")
			return
		}
		log.Error("sendRun write msg to conn error", err)
		dashboard.Overview.SentErrClientCnt.Add(1)
		return
	}

	return
}

//Send 将消息写入发送channel
//如果channel的buffer满后，会阻塞
func (this *Channel) Send(msg packets.ControlPacket) (err error) {
	if this.IsStop() {
		log.Debug("channel is closed")
		return
	}
	this.iSendList.Push(msg)
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
}

func (this *Channel) IsStop() bool {
	return atomic.LoadInt32(&this.isStoped) > 0
}
