# mqtt-broker
一个高并发，高性能，高可靠的mqtt服务器

[TOC]


## TODO:
- [x]1. Session的发送窗口，即发送窗口满时，不能发其它消息
- [ ]2. Retain消息发送给有订阅此消息主题的新客户端
- [ ]3. CleanSession处理
- [x]4. 实现websocket
- [x]5. tls支持
- [ ]6. 遗嘱处理
- [ ]7. 登录验证
- [ ]8. Session做为客户端使用,benchmark中使用Session
- [ ]9. dashboard页面实现
- [ ]10. 实现管理配置后台
- [ ]11. 发布1.0版本
- [ ]12. sqlite3 store支持
- [ ]13. 实现集群
- [ ]14. 实现多集群管理
- [ ]15. 服务器监控
- [ ]16. 发布2.0版本

#Session操作流程

##PUBLISH qos=0 
```sequence
send->recv:PUBLISH qos=0
send->session:onPublicDone
```


##PUBLISH qos=1 
```sequence
send->recv:PUBLISH qos=1
recv-->send:PUBACK
send->session:onPublicDone
send->other:PUBLISH topic 
```
##PUBLISH qos=2
```sequence
send->recv:PUBLISH qos=2
recv-->send:PUBREC
send->send:保存消息,
send->recv:PUBREL
recv-->send:PUBCOMP
send->send:删除消息
send->session:onPublicDone
send->other:PUBLISH topic 
```

##给session发送消息
```sequence
other->session:Publish
session->session:checkInflightList
```

##checkInflightList
```flow
st=>start: 开始
e=>end: 结束
inflightcheck=>condition: inflight队列满?
chanel.send=>operation: 调用Channel发送一个包
insertpendding=>operation: 插入等待队列
insertinfilight=>operation: 将PUBLISH消息插入inflight队列
st->inflightcheck
inflightcheck(yes)->insertpendding->e
inflightcheck(no)->chanel.send->insertinfilight->e
```
##onTick
```flow
st=>start: 开始
e=>end: 结束
loopInflightList=>operation: 遍历inflight列表
loopInflightListIsEnd=>condition: 遍历完成?
condMsgTimeout=>condition: 消息是否超时?
saveTimeoutMsg=>operation: 记录超时消息
loopTimeoutMsg=>operation: 遍历超时消息
loopTimeoutMsgIsEnd=>condition: 遍历完成?
chanelresend=>operation: 调用Channel重发超时包

st->loopInflightList->loopInflightListIsEnd
loopInflightListIsEnd(no)->condMsgTimeout
loopInflightListIsEnd(yes)->loopTimeoutMsg->loopTimeoutMsgIsEnd
condMsgTimeout(yes)->saveTimeoutMsg(left)->loopInflightList
condMsgTimeout(no)->loopInflightList
loopTimeoutMsgIsEnd(yes)->chanelresend->e
loopTimeoutMsgIsEnd(no)->loopTimeoutMsg
```