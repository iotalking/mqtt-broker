package session

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"

	log "github.com/Sirupsen/logrus"

	"github.com/iotalking/mqtt-broker/config"
	"github.com/iotalking/mqtt-broker/dashboard"
	"github.com/iotalking/mqtt-broker/store"
)

//处理 CONNECT 消息
//连接消息
func (this *Session) onConnect(msg *packets.ConnectPacket) (err error) {
	//如果已经连接，则是违规，应该断开
	if this.IsConnected() {
		log.Error("connected client dup CONNECT")
		return packets.ConnErrors[packets.ErrProtocolViolation]
	}
	//判断保留标志位是否为0,如果不为0，返回协议错误
	if msg.ReservedBit&0x1 == 0x1 {
		log.Error("client ReservedBit != 0")
		return packets.ConnErrors[packets.ErrProtocolViolation]
	}
	//检查是否有相同clientId的连接，如果有，则要断开原有连接
	this.clientId = msg.ClientIdentifier
	this.mgr.DisconectSessionByClientId(this.clientId)

	conack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	conack.ReturnCode = packets.Accepted
	if msg.CleanSession {
		conack.SessionPresent = false
	} else {
		//如果服务器有保存session,则 SessionPresent设成true
		//如果服务器没有保存session,则SessionPresent设成false
		conack.SessionPresent = false
	}

	log.Debug("sessionMgr.OnConnected")
	this.mgr.OnConnected(this)

	log.Debug("session.Send")

	err = this.channel.Send(conack)
	atomic.StoreInt32(&this.connected, 1)

	if msg.WillFlag {
		this.willFlag = true
		this.willTopic = msg.WillTopic
		this.willQos = msg.WillQos
		this.willMessage = msg.WillMessage
	}
	//save keep alive (seconds)

	atomic.StoreInt64(&this.timeout, int64(float64(this.timeout)*1.5*float64(time.Second)))
	return
}

//处理 CONNACK 消息
//连接应答消息
//此消息只有作为客户端时才会收到
func (this *Session) onConnack(msg *packets.ConnackPacket) error {

	if msg.ReturnCode > 0 {
		log.Debug("server return conack code is:", msg.ReturnCode)
		atomic.StoreInt32(&this.connected, -int32(msg.ReturnCode))
	} else {
		log.Debug("Session was connected")
		atomic.StoreInt32(&this.connected, 1)
	}
	this.mgr.OnConnected(this)
	if this.connectToken != nil {
		if msg.ReturnCode != packets.Accepted {
			//验证失败
			this.connectToken.err = packets.ConnErrors[msg.ReturnCode]
		}
		this.connectToken.flowComplete()
	}

	return nil
}

//由独立的协程调用，注意共享数据访问
func (this *Session) BroadcastSessionInfo() {

	subMgr := this.mgr.GetSubscriptionMgr()
	infoTopic := config.SessionInfoTopic(this.clientId)
	sessionQosMap, err := subMgr.GetSessions(infoTopic)
	if err != nil {
		log.Error("subMgr.GetSessions error:", err)
		return
	}
	if len(sessionQosMap) == 0 {
		log.Info("sessionQosMap is empty for topic:", infoTopic)
		return
	}
	nmsg := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	var info SessionInfo
	info.Id = this.clientId
	info.InflightMsgCnt = this.inflightingList.Len()
	info.PeddingMsgCnt = this.peddingMsgList.Len()
	info.SendingMsgCnt = this.channel.iSendList.Len()
	bs, err := json.MarshalIndent(info, "", "\t")
	log.Debug("broadcastSessionInfo ", info)

	if err != nil {
		log.Error("broadcastSessionInfo json.MarshalIndent :", err)
		return
	}
	nmsg.Payload = bs
	//qos取订阅和原消息的最小值
	nmsg.Qos = 0

	for v, _ := range sessionQosMap {
		s := v.(*Session)
		if s.IsConnected() && !s.IsClosed() {
			_, err = s.Publish(nmsg)
			if err != nil {
				log.Errorf("session[%s].Publish Send error:", s.clientId, err)
				break
			}
		} else {
			log.Debug("session is disconnected or not connect")
		}

	}

	return
}

//要除自己之外的所有订阅者
func (this *Session) sendToSubcriber(msg *packets.PublishPacket) error {
	log.Debug("sendToSubcriber")
	subMgr := this.mgr.GetSubscriptionMgr()

	sessionQosMap, err := subMgr.GetSessions(msg.TopicName)
	if err != nil {
		log.Error("subMgr.GetSessions error:", err)
		return err
	}
	if len(sessionQosMap) == 0 {
		log.Debug("sessionQosMap is empty for topic:", msg.TopicName)
	}
	log.Debug("sessionQosMap len:", len(sessionQosMap))
	msg.Retain = false
	for v, qos := range sessionQosMap {
		s := v.(*Session)
		if s != this && s.IsConnected() && !s.IsClosed() {
			nmsg := msg.Copy()
			//qos取订阅和原消息的最小值
			if qos < msg.Qos {
				nmsg.Qos = qos
			}
			_, err = s.Publish(msg)
			if err != nil {
				log.Errorf("session[%s].Publish Send error:", s.clientId, err)
				break
			}
		} else {
			log.Debug("session is disconnected or not connect")
		}

	}

	return nil
}

//处理 PUBLISH 消息
//发布消息
//retain消息的处理方式：
//1.服务端只保存payload不为空的消息
//2.服务端收到payload为空的消息清空以前相关主题的消息
func (this *Session) onPublish(msg *packets.PublishPacket) (err error) {
	log.Debug("session onpublish")
	//从mgr获取和tipic匹配的sessions
	//TODO:获取匹配的sessions
	payloadLen := len(msg.Payload)
	if payloadLen == 0 {
		if msg.Retain {
			err = this.mgr.GetStoreMgr().RemoveMsgByTopic(msg.TopicName)
			if err != nil {
				log.Errorf("this.mgr.storeMgr.RemoveMsgByTopic error:", err)
				if err == store.ErrNoExsit {
					err = nil
				} else {
					return
				}
			}
		}

	}
	switch msg.Qos {
	case 0:
		if msg.Retain {
			//清空离线消息
			if payloadLen > 0 {
				err = this.mgr.GetStoreMgr().RemoveMsgByTopic(msg.TopicName)
				if err != nil {
					log.Errorf("this.mgr.storeMgr.RemoveMsgByTopic error:", err)
					if err == store.ErrNoExsit {
						err = nil
					}
				}
				log.Debugf("retain msg has clear for client:%s", this.clientId)

				err = this.mgr.GetStoreMgr().SaveRetainMsg(msg.TopicName, msg.Payload, msg.Qos)
				if err != nil {
					log.Errorf("SaveRetainMsg error:%s", err.Error())
				}
			}

		}
		dashboard.Overview.RecvMsgCnt.Add(1)
		if this.isServer {
			this.sendToSubcriber(msg)
		} else {
			//回调接入到消息的函数
			this.callbackOnMessage(msg)
		}
	case 1:
		ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
		ack.MessageID = msg.MessageID
		this.channel.Send(ack)
		dashboard.Overview.RecvMsgCnt.Add(1)
		smsg := &store.Msg{
			ClientId: this.clientId,
			MsgId:    msg.MessageID,
			Topic:    msg.TopicName,
			Qos:      msg.Qos,
			Body:     msg.Payload,
		}

		if msg.Retain {
			if payloadLen > 0 {
				err = this.mgr.GetStoreMgr().SaveRetainMsg(smsg.Topic, smsg.Body, smsg.Qos)
				if err != nil {
					log.Error(err)
					err = packets.ConnErrors[packets.ErrRefusedServerUnavailable]
					return
				}
			}

		}

		if this.isServer {
			this.sendToSubcriber(msg)
		} else {
			//回调接入到消息的函数
			this.callbackOnMessage(msg)
		}
	case 2:
		ack := packets.NewControlPacket(packets.Pubrec).(*packets.PubrecPacket)
		ack.MessageID = msg.MessageID
		this.channel.Send(ack)

		storeMgr := this.mgr.GetStoreMgr()
		if _, err = storeMgr.GetMsgByClientIdMsgId(this.clientId, msg.MessageID); err == store.ErrNoExsit {
			//保存消息，等待PUBREL后再发送
			smsg := &store.Msg{
				ClientId: this.clientId,
				MsgId:    msg.MessageID,
				Topic:    msg.TopicName,
				Qos:      msg.Qos,
				Body:     msg.Payload,
			}
			err = storeMgr.SaveByClientIdMsgId(smsg)
			if err != nil {
				log.Error(err)
				err = packets.ConnErrors[packets.ErrRefusedServerUnavailable]
				return
			}
			if msg.Retain {
				if payloadLen > 0 {
					err = this.mgr.GetStoreMgr().SaveRetainMsg(smsg.Topic, smsg.Body, smsg.Qos)
					if err != nil {
						log.Error(err)
						err = packets.ConnErrors[packets.ErrRefusedServerUnavailable]
						return
					}
				}

			}
		}

	}
	return err
}

//处理 PUBACK 消息
//向qos=1的主题消息发布者应答
func (this *Session) onPuback(msg *packets.PubackPacket) error {
	this.onInflightDone(msg)
	return nil
}

//处理 PUBREC 消息
func (this *Session) onPubrec(msg *packets.PubrecPacket) error {

	relmsg := packets.NewControlPacket(packets.Pubrel).(*packets.PubrelPacket)
	relmsg.MessageID = msg.MessageID
	this.channel.Send(relmsg)

	this.updateInflightMsg(relmsg)
	return nil
}

//处理 PUBREL 消息
//向qos=2的主题消息发布者应答服务器第二步
//对PUBREC的响应
func (this *Session) onPubrel(msg *packets.PubrelPacket) error {
	compmsg := packets.NewControlPacket(packets.Pubcomp).(*packets.PubcompPacket)
	compmsg.MessageID = msg.MessageID
	this.channel.Send(compmsg)

	storeMgr := this.mgr.GetStoreMgr()
	smsg, err := storeMgr.GetMsgByClientIdMsgId(this.clientId, msg.MessageID)
	if err != nil {
		log.Debugf("onPubrel error:%s,clientId:%s,msgId:%d", err, this.clientId, msg.MessageID)
		return nil
	}
	dashboard.Overview.RecvMsgCnt.Add(1)

	storeMgr.RemoveMsgById(smsg.Id)

	pubMsg := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	pubMsg.Qos = smsg.Qos
	pubMsg.Payload = smsg.Body
	pubMsg.TopicName = smsg.Topic

	if this.isServer {
		this.sendToSubcriber(pubMsg)
	} else {
		//回调接入到消息的函数
		this.callbackOnMessage(pubMsg)
	}
	return nil
}

//处理 PUBCOMP 消息
//向qos=2的主题消息发布者应答服务器第三步(发布完成)
//对PUBREL的响应
//它是QoS 2等级协议交换的第四个也是最后一个报文
func (this *Session) onPubcomp(msg *packets.PubcompPacket) error {
	this.onInflightDone(msg)
	return nil
}

//处理 SUBSCRIBE - 订阅主题消息
//客户端向服务端发送SUBSCRIBE报文用于创建一个或多个订阅。
//每个订阅注册客户端关心的一个或多个主题。为了将应用消息转发给与那些订阅匹配的主题，服务端发送PUBLISH报文给客户端。
//SUBSCRIBE报文也（为每个订阅）指定了最大的QoS等级，服务端根据这个发送应用消息给客户端。
func (this *Session) onSubscribe(msg *packets.SubscribePacket) error {
	log.Debug("session.onSubscribe")
	submgr := this.mgr.GetSubscriptionMgr()

	err := submgr.Add(msg.Topics, msg.Qoss, this)

	suback := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
	suback.MessageID = msg.MessageID

	suback.ReturnCodes = msg.Qoss
	for i, _ := range msg.Qoss {
		if err != nil {
			suback.ReturnCodes[i] = 0x80
		}

	}
	//向客户端发送匹配主题的retain消息
	go this.procRetainMessages(msg)

	return this.channel.Send(suback)
}
func (this *Session) procRetainMessages(msg *packets.SubscribePacket) {
	subMgr := this.mgr.GetSubscriptionMgr()
	storeMgr := this.mgr.GetStoreMgr()
	msgs, err := storeMgr.GetAllRetainMsgs()
	if err != nil {
		log.Debug("no retain messages")
		return
	}

	log.Debug("retain len:", len(msgs))
	for _, msg := range msgs {
		pubMsg := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
		log.Debugf("%#v", msg)
		pubMsg.Retain = true
		pubMsg.TopicName = msg.Topic
		pubMsg.Qos = msg.Qos
		pubMsg.Payload = msg.Body
		if subMgr.IsTopicMatch(msg.Topic, msg.Topic) {
			//匹配主题，发送给session
			this.Publish(pubMsg)
		}
	}
}

//处理 SUBACK – 订阅确认
//服务端发送SUBACK报文给客户端，用于确认它已收到并且正在处理SUBSCRIBE报文。
//SUBACK报文包含一个返回码清单，它们指定了SUBSCRIBE请求的每个订阅被授予的最大QoS等级。
func (this *Session) onSuback(msg *packets.SubackPacket) error {
	this.onInflightDone(msg)
	return nil
}

//处理 UNSUBACK –取消订阅
//客户端发送UNSUBSCRIBE报文给服务端，用于取消订阅主题。
func (this *Session) onUnsubscribe(msg *packets.UnsubscribePacket) error {
	submgr := this.mgr.GetSubscriptionMgr()
	for _, s := range msg.Topics {
		submgr.Remove(s, this)
	}
	ack := &packets.UnsubackPacket{
		MessageID: msg.MessageID,
	}
	this.channel.Send(ack)
	return nil
}

//处理 UNSUBSCRIBE –取消订阅确认
//服务端发送UNSUBACK报文给客户端。
func (this *Session) onUnsuback(msg *packets.UnsubackPacket) error {
	this.onInflightDone(msg)
	return nil
}

//处理 PINGREQ
func (this *Session) onPingreq(msg *packets.PingreqPacket) error {
	pingresp := packets.NewControlPacket(packets.Pingresp).(*packets.PingrespPacket)
	this.channel.Send(pingresp)
	return nil
}

//处理 PINGRESP
//服务端发送PINGRESP报文响应客户端的PINGREQ报文。表示服务端还活着。
//保持连接（Keep Alive）处理中用到这个报文
func (this *Session) onPingresp(msg *packets.PingrespPacket) error {
	//TODO
	return nil
}

//处理 DISCONNECT
//从sessionMgr中删除session
//断开网络
func (this *Session) onDisconnect(msg *packets.DisconnectPacket) error {
	log.Debugf("session(%s) onDisconnect", this.clientId, this.sentMsgChan)
	this.mgrOnDisconnected()

	log.Debugf("session(%s) onDisconnect end", this.clientId)
	return nil
}

//消息发送完成。走完所有流程
func (this *Session) onInflightDone(msg packets.ControlPacket) {
	switch msg.Type() {
	case packets.Publish:
		if msg.Details().Qos > 0 {
			break
		}
		fallthrough
	case packets.Puback, packets.Pubcomp, packets.Suback, packets.Unsuback:
		imsg := this.removeInflightMsg(msg.Details().MessageID)
		if imsg != nil && imsg.pt != nil && imsg.pt.t != nil {
			switch msg.Type() {
			case packets.Suback:
				var subToken *SubscribeToken = imsg.pt.t.(*SubscribeToken)
				subackMsg := msg.(*packets.SubackPacket)
				for i, sub := range subToken.subs {
					subToken.subResult[sub] = subackMsg.ReturnCodes[i]
				}
			}
			log.Debug("flowComplete for msgId:", imsg.pt.p.Details().MessageID)
			imsg.pt.t.flowComplete()
		}
	}

	if this.inflightingList.Len() < config.MaxSizeOfInflight {
		select {
		case <-this.peddingChan:
		default:
		}
		v := this.peddingMsgList.Pop()
		if v != nil {
			this.insert2Inflight(v.(*PacketAndToken))
		}
	}
	dashboard.Overview.SentMsgCnt.Add(1)
}
