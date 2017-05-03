package config

import (
	"time"
)

//restful api port
var RestfulPort = 8081

var DashboordUrl = "/dashboard"
var DashboordApiUrl = DashboordUrl + "/api"

//发送心跳包的周期（秒）
var PingTimeout int64 = 30

//客户端用户检测PINGRESP有没有超时接收
var PingrespTimeout int64 = 45 //1.5 PingTimeout

//发送channel的buffer的最大长度
var MaxSizeOfSendChannel int = 1

//接入channel的buffer的最大长度
var MaxSizeOfRecvChannel int = 100

//sessionMgr处理新连接的chan缓冲大小
var MaxSizeOfNewConnectionChan int = 10000

//连接超时时间,秒
var ConnectTimeout time.Duration = time.Duration(3)

//消息发送超时时间
var SentTimeout int64 = 5

//重发消息的最大次数
var MaxRetryTimes int = 5

//SessionMgr publishMsg 的最大缓存数
var MaxSizeOfPublishMsg = 100 * 1024
