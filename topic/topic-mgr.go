package topic

type SessoinQosMap map[interface{}]byte

//订阅管理器
type SubscriptionMgr interface {
	//添加订阅
	//subs和qoss根据索引位置关联
	Add(subs []string, qoss []byte, session interface{}) error
	//删除订阅
	//如果session!=nil，那么删除指定订阅下的session
	//如果session==nil,那么删除所以指定订阅
	Remove(filter string, session interface{}) error

	//将session从所有关联订阅中删除
	RemoveSession(session interface{})
	//返回匹配主题的session列表
	GetSessions(topic string) (SessoinQosMap, error)
}

func NewSubscriptionMgr() SubscriptionMgr {
	return newNode()
}
