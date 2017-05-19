package store

type Error int

const (
	//消息重复
	ErrDupMsg  Error = 1
	ErrNoExsit Error = 2
)

type Msg struct {
	//Store里唯一的消息标识
	Id       int
	ClientId string
	//消息作者定义的id
	MsgId uint16
	Topic string
	Qos   byte
	Body  []byte
}

//发送中的消息存储
type InflightStore interface {
	//以ClientId&MsgId作key
	SaveByClientIdMsgId(msg *Msg) error
	//以Topic作为key
	SaveRetainMsg(topic string, body []byte, qos byte) error

	GetMsgByClientIdMsgId(clientId string, msgId uint16) (msg *Msg, err error)
	//根据消息Id删除消息
	RemoveMsgById(id int) error
}

//相关题的最后一条离线消息存储
type RetainStore interface {
	//Retain消息
	//获取所有离线消息
	GetAllRetainMsgs() (msgs []*Msg, err error)

	//删除topic的离线消息
	RemoveMsgByTopic(topic string) error
}
type StoreMgr interface {
	InflightStore
	RetainStore
}

func (this Error) Error() string {
	switch this {
	case ErrDupMsg:
		return "ErrDupMsg"
	case ErrNoExsit:
		return "ErrNoExsit"
	}
	return "Unknown"
}

type ProviderFactory func() StoreMgr

var factory ProviderFactory

func NewStoreMgr() StoreMgr {
	if factory == nil {
		panic("register provider first")
	}
	return factory()
}

func RegisterProvider(f func() StoreMgr) {
	if factory != nil {
		panic("dup factory")
	}
	factory = f
}
