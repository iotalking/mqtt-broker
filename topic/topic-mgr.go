package topic

type Session interface {
}

//订阅管理器
type SubscriptionMgr struct {
}

func NewSubscriptionMgr() *SubscriptionMgr {
	return &SubscriptionMgr{}
}

//添加订阅
//tipics和qoss根据索引位置关联
func (this *SubscriptionMgr) AddFilter(tipics []string, qoss []byte, session *Session) {

}

//删除订阅
//如果session!=nil，那么删除指定订阅filter的session
//如果session==nil,那么删除所以指定订阅filter
func (this *SubscriptionMgr) RemoveFilter(filter string, session *Session) {

}

//返回匹配主题的session列表
func (this *SubscriptionMgr) MatchTipic(tipic string) []*Session {
	return nil
}
