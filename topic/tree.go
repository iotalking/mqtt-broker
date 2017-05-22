package topic

import (
	"sync"
	//	"fmt"
	"strings"
)

type treeSubsMgr struct {
	//如果没有子节点时，存储session和对应的qos
	sessionsMap SessoinQosMap

	//存储此级所有字节的name及对应的子节点
	subsMap map[string]*treeSubsMgr

	mux sync.Mutex
}

func newNode() *treeSubsMgr {
	return &treeSubsMgr{
		sessionsMap: make(SessoinQosMap),
		subsMap:     make(map[string]*treeSubsMgr),
	}
}

//将一个订阅添加到树中，并关联qos和session
func (this *treeSubsMgr) add(filter string, qos byte, session interface{}) {
	this.insert(strings.Split(filter, "/"), 0, qos, session)
}

//将分好级的段递归插入
//cur :当前级别，< fields的长度
func (this *treeSubsMgr) insert(fields []string, cur int, qos byte, session interface{}) {
	var treeSubsMgr *treeSubsMgr
	var ok bool
	if treeSubsMgr, ok = this.subsMap[fields[cur]]; !ok {
		treeSubsMgr = newNode()
		this.subsMap[fields[cur]] = treeSubsMgr
	}
	if cur == (len(fields) - 1) {
		//叶节点
		treeSubsMgr.sessionsMap[session] = qos
		return
	} else {
		treeSubsMgr.insert(fields, cur+1, qos, session)
	}

}

//将订阅从树中删除
//如果session != nil时，只删除filter下的这个session
//如果session == nil,则将订阅从树中完全删除，包括子节点
func (this *treeSubsMgr) del(filter string, session interface{}) {
	this.remove(strings.Split(filter, "/"), 0, session)
}

func (this *treeSubsMgr) remove(fields []string, cur int, session interface{}) {
	var treeSubsMgr *treeSubsMgr
	var ok bool
	if treeSubsMgr, ok = this.subsMap[fields[cur]]; !ok {
		return
	}
	if cur == (len(fields) - 1) {
		//叶节点
		if session == nil {
			//全部删除节点
			delete(treeSubsMgr.subsMap, fields[cur])
		} else {
			//删除节点中的session
			delete(treeSubsMgr.sessionsMap, session)
		}

		return
	} else {
		treeSubsMgr.remove(fields, cur+1, session)
	}
}

//通过主题返回所有匹配的session
func (this *treeSubsMgr) getSessions(topic string) SessoinQosMap {
	rm := make(SessoinQosMap)
	this._getSessions([]byte(topic), func(sm SessoinQosMap) {
		for k, v := range sm {
			rm[k] = v
		}
	})
	return rm
}

func (this *treeSubsMgr) _getSessions(fields []byte, cb func(SessoinQosMap)) {
	var treeSubsMgr *treeSubsMgr
	var ok bool
	var l = len(fields)
	var field = string(fields)
	var i int
	if l <= 0 {
		return
	}
	for i = 0; i < l; i++ {
		if fields[i] == '/' {
			field = string(fields[:i])
			fields = fields[i+1:]
			break
		}
	}

	if treeSubsMgr, ok = this.subsMap["#"]; ok {
		//如果有"#"，则直接匹配
		cb(treeSubsMgr.sessionsMap)

	}
	if treeSubsMgr, ok = this.subsMap["+"]; ok {

		if i == l {
			//叶节点
			cb(treeSubsMgr.sessionsMap)
		} else {
			treeSubsMgr._getSessions(fields, cb)
		}
	}

	if treeSubsMgr, ok = this.subsMap[field]; ok {

		if i == l {
			//叶节点
			cb(treeSubsMgr.sessionsMap)
		} else {
			treeSubsMgr._getSessions(fields, cb)
		}
	}

	return
}

//添加订阅
//subs和qoss根据索引位置关联
func (this *treeSubsMgr) Add(subs []string, qoss []byte, session interface{}) error {
	this.mux.Lock()
	defer this.mux.Unlock()
	for i, s := range subs {
		this.add(string(s), qoss[i], session)
	}
	return nil
}

//删除订阅
//如果session!=nil，那么删除指定订阅下的session
//如果session==nil,那么删除所以指定订阅
func (this *treeSubsMgr) Remove(filter string, session interface{}) error {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.del(filter, session)
	return nil
}

//将session从所有关联订阅中删除
func (this *treeSubsMgr) RemoveSession(session interface{}) {
	this.mux.Lock()
	defer this.mux.Unlock()
	if session == nil {
		panic("session is nil")
	}
	delete(this.sessionsMap, session)
	for _, n := range this.subsMap {
		n.RemoveSession(session)
	}
	return
}

//返回匹配主题的session列表
func (this *treeSubsMgr) GetSessions(topic string) (SessoinQosMap, error) {
	this.mux.Lock()
	defer this.mux.Unlock()
	sqm := this.getSessions(topic)
	return sqm, nil
}

//判断主题和订阅是否匹配
func (this *treeSubsMgr) IsTopicMatch(topic string, subscription string) bool {
	//TODO
	//要做subscription合法性校验
	topicLen := len(topic)
	subLen := len(subscription)
	i := 0
	j := 0
	for i < subLen && j < topicLen {
		//如果字体相等则比较下一个,
		//如果不等：
		//如果subscription[i]是“+”,那么topic直接查找'/‘或者结尾,之间的字体都认为是匹配的
		//如果subscription[i]是"#",那么topic后面的字符可以忽略，认为匹配
		//其它不匹配
		c := subscription[i]
		if c == topic[i] {
			i++
			j++
			continue
		}
		switch c {
		case '+':
			for j < topicLen {
				if topic[j] == '/' {
					break
				}
				j++
			}
			i++
		case '#':
			return true
		default:
			return false
		}
	}
	if i != subLen || j != topicLen {
		//处理topic比subscription短的情况
		if j == topicLen {
			tail := string(subscription[i:])
			switch tail {
			case "+", "#", "/+", "/#":
				return true
			}
		}
		return false
	}
	return true
}
func (this *treeSubsMgr) IsSubscriptionValid(sub string) {

}
