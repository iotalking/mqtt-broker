package client

import (
	"crypto/tls"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/iotalking/mqtt-broker/config"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/safe-runtine"
	"github.com/iotalking/mqtt-broker/session"
	"github.com/iotalking/mqtt-broker/store"
	"github.com/iotalking/mqtt-broker/topic"

	_ "github.com/iotalking/mqtt-broker/store/mem-provider"
)

var mgrOnce sync.Once
var sessionMgr session.SessionMgr

type Client struct {
	session     *session.Session
	proto       string
	addr        string
	user        string
	password    []byte
	clientId    string
	WillTopic   string
	WillMessage []byte
	WillQos     byte
	WillRetain  bool
	Keepalive   uint16

	mainRuntine *runtine.SafeRuntine
}

func NewClient(id string, mgr session.SessionMgr) *Client {
	mgrOnce.Do(func() {
		sessionMgr = mgr
	})
	c := &Client{
		clientId: id,
	}
	return c
}

//连接服务器
//proto:mqtt,mqtts,ws,wss
//mqtt:tcp
//mqtt:tcp tls
//ws:websocket
//wss:websocket tls
//addr格式：
//[username][:password]@ip[:port]
func (this *Client) Connect(proto, addr string) (token session.Token, err error) {
	this.proto = proto
	this.addr = addr
	//解析username和password
	tmps := strings.Split(addr, "@")
	if len(tmps) > 1 {
		addr = tmps[1]
		//包含用户名和密码段
		tmps := strings.Split(tmps[0], ":")
		this.user = tmps[0]
		if len(tmps) > 1 {
			this.password = []byte(tmps[1])
		}

	}
	var c io.ReadWriteCloser
	switch proto {
	case "mqtt":
		c, err = this.newTcpConn(addr)
	case "mqtts":
		c, err = this.newTcpTlsConn(addr)
	default:
		c, err = this.newTcpConn(addr)
	}

	if err == nil {
		this.session = session.NewSession(this, c, false)
		this.session.SetClientId(this.clientId)
		sessionMgr.HandleConnection(this.session)

		connectMsg := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
		connectMsg.ProtocolName = "MQTT"
		connectMsg.ProtocolVersion = 4
		connectMsg.Username = this.user
		connectMsg.Password = this.password
		connectMsg.ClientIdentifier = this.clientId
		token, err = this.session.Send(connectMsg)
	} else {
		log.Error("connect error:", err)
		defer func() {
			recover()
		}()

	}
	return
}

func (this *Client) newTcpConn(addr string) (c io.ReadWriteCloser, err error) {
	c, err = net.DialTimeout("tcp", addr, time.Duration(config.ConnectTimeout))
	return
}

func (this *Client) newTcpTlsConn(addr string) (c io.ReadWriteCloser, err error) {
	var config tls.Config
	config.InsecureSkipVerify = true
	c, err = tls.Dial("tcp", addr, &config)
	return
}

func (this *Client) Disconnect() (err error) {
	if this.session == nil || this.session.IsClosed() {
		return
	}
	disconnectMsg := packets.NewControlPacket(packets.Disconnect).(*packets.DisconnectPacket)
	token, err := this.session.Send(disconnectMsg)
	if err == nil {
		token.Wait()
		log.Debug("disconnect token.Wait return")
	}
	return
}

//发布消息
func (this *Client) Publish(topic string, body []byte, qos byte, retain bool) (token session.Token, err error) {
	msg := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	msg.TopicName = topic
	msg.Payload = body
	msg.Qos = qos
	msg.Retain = retain
	token, err = this.session.Publish(msg)
	return
}

//订阅主题,可以一次订阅多条
//submap
//key:subscription
//value:qos
func (this *Client) Subcribe(submap map[string]byte) (token session.Token, err error) {
	subMsg := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
	for sub, qos := range submap {
		subMsg.Topics = append(subMsg.Topics, sub)
		subMsg.Qoss = append(subMsg.Qoss, qos)
	}
	token, err = this.session.Send(subMsg)
	return
}

//不订阅主题
func (this *Client) Unsubcribe(subs ...string) (token session.Token, err error) {
	unsubMsg := packets.NewControlPacket(packets.Unsubscribe).(*packets.UnsubscribePacket)
	unsubMsg.Topics = subs
	token, err = this.session.Send(unsubMsg)
	return
}
func (this *Client) SetOnMessage(cb func(topic string, body []byte, qos byte)) {
	this.session.SetOnMessage(func(msg *packets.PublishPacket) {
		if cb != nil {
			cb(msg.TopicName, msg.Payload, msg.Qos)
		}

	})
}

func (this *Client) SetOnDisconnected(cb func()) {
	this.session.SetOnDisconnected(cb)
}
func (this *Client) HandleConnection(session *session.Session) {
	sessionMgr.HandleConnection(session)
	return
}
func (this *Client) OnConnected(session *session.Session) {
	sessionMgr.OnConnected(session)
}
func (this *Client) OnConnectTimeout(session *session.Session) {
	sessionMgr.OnConnectTimeout(session)
}
func (this *Client) OnDisconnected(session *session.Session) {
	sessionMgr.OnDisconnected(session)
}
func (this *Client) DisconectSessionByClientId(clientId string) {
	sessionMgr.DisconectSessionByClientId(clientId)
}
func (this *Client) GetSubscriptionMgr() topic.SubscriptionMgr {
	return sessionMgr.GetSubscriptionMgr()
}
func (this *Client) GetSessions() dashboard.SessionList {
	return sessionMgr.GetSessions()
}
func (this *Client) GetStoreMgr() store.StoreMgr {
	return sessionMgr.GetStoreMgr()
}
