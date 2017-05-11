# mqtt-broker
一个高并发，高性能，高可靠的mqtt服务器

##TODO:
	Session的发送窗口，即发送窗口中小满时，不能发其它消息
    Retain消息发送给有订阅此消息主题的新客户端
    CleanSession处理
    实现websocket [ok]
    tls支持 [ok]
    遗嘱处理
    登录验证
    Session做为客户端使用,benchmark中使用Session
    dashboard页面实现
    实现管理配置后台
    发布1.0版本
    sqlite3 store支持
    实现集群
    实现多集群管理
    服务器监控
    发布2.0版本