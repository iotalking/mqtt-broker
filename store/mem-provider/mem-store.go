package memstore

import (
	"fmt"
	"sync"

	"github.com/iotalking/mqtt-broker/store"
)

type memStore struct {

	//Id索引
	indexId map[int]*store.Msg

	//clientId&MsgId索引
	//key:clientId拼接MsgId的字串
	//如:clientId="client123" MsgId=100 那么key="client123100"
	indexClientIdMsgId map[string]*store.Msg

	//以topic为索引存储离线消息
	retainMsgs map[string]*store.Msg

	mtx       sync.Mutex
	retainMtx sync.Mutex
}

func (this *memStore) ClientIdMsgIdAsKey(clientId string, msgId uint16) string {
	return fmt.Sprintf("%s%d", clientId, msgId)
}
func (this *memStore) SaveByClientIdMsgId(msg *store.Msg) error {
	this.mtx.Lock()
	defer this.mtx.Unlock()
	key := this.ClientIdMsgIdAsKey(msg.ClientId, msg.MsgId)
	if _, ok := this.indexClientIdMsgId[key]; ok {
		return store.ErrDupMsg
	}
	msg.Id = len(this.indexId)

	this.indexId[msg.Id] = msg
	this.indexClientIdMsgId[key] = msg

	return nil
}

//以Topic作为key
func (this *memStore) SaveRetainMsg(topic string, body []byte, qos byte) error {
	this.mtx.Lock()
	defer this.mtx.Unlock()
	msg := &store.Msg{
		Topic: topic,
		Body:  body,
		Qos:   qos,
	}
	if len(msg.Body) == 0 {
		//不能存储内容为空的消息，反而会删除相同主题的retain消息
		delete(this.retainMsgs, msg.Topic)
	} else {
		msg.ClientId = ""
		msg.MsgId = 0
		this.retainMsgs[msg.Topic] = msg
	}

	return nil
}
func (this *memStore) GetMsgByClientIdMsgId(clientId string, msgId uint16) (msg *store.Msg, err error) {
	this.mtx.Lock()
	defer this.mtx.Unlock()
	msg, ok := this.indexClientIdMsgId[this.ClientIdMsgIdAsKey(clientId, msgId)]
	if !ok {
		return nil, store.ErrNoExsit
	}
	return msg, nil
}

//根据消息Id删除消息
func (this *memStore) RemoveMsgById(id int) error {
	this.mtx.Lock()
	defer this.mtx.Unlock()
	msg, ok := this.indexId[id]
	if ok {
		delete(this.indexId, id)
		delete(this.indexClientIdMsgId, this.ClientIdMsgIdAsKey(msg.ClientId, msg.MsgId))
	} else {
		return store.ErrNoExsit
	}
	return nil
}

//获取所有离线消息
func (this *memStore) GetAllRetainMsgs() (msgs []*store.Msg, err error) {
	this.retainMtx.Lock()
	defer this.retainMtx.Unlock()
	for _, msg := range this.retainMsgs {
		msgs = append(msgs, msg)
	}
	return
}
func (this *memStore) RemoveMsgByTopic(topic string) error {
	this.retainMtx.Lock()
	defer this.retainMtx.Unlock()
	if _, ok := this.retainMsgs[topic]; !ok {
		return store.ErrNoExsit
	}
	delete(this.retainMsgs, topic)
	return nil
}

func NewStoreMgr() store.StoreMgr {
	return &memStore{
		indexId:            make(map[int]*store.Msg),
		indexClientIdMsgId: make(map[string]*store.Msg),
		retainMsgs:         make(map[string]*store.Msg),
	}
}

func init() {
	store.RegisterProvider(NewStoreMgr)
}
