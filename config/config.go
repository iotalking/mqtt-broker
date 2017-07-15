package config

import (
	"time"
)

var TcpPort = 1883

//tcp ssl 加密端口
var TcpTLSPort = 8883

//websocket端口,url ip:port/mqtt
var WSPort = 8083

//websocket ssl加密商品,url ip:port/mqtts
var WSSPort = 8084

var CertFileName = "./mqttdebug.crt"
var KeyFileName = "./mqttdebug.key"

//pprof查看端口,url ip:port/debug/pprof
var PprofPort = 6060

//web server for some html page
var WebPort = 8080

//restful api port,url base:  ip:port/dashboard
var RestfulPort = 8081

var DashboordUrl = "/dashboard"
var DashboordApiUrl = DashboordUrl + "/api"
var RestfullApiUrl = "/api"

//发送心跳包的周期（秒）
var PingTimeout = 30 * int64(time.Second)

//客户端用户检测PINGRESP有没有超时接收
var PingrespTimeout = 45 * int64(time.Second) //1.5 PingTimeout

//连接超时时间,秒
var ConnectTimeout = 10 * int64(time.Second)

//消息发送超时时间
var SentTimeout = 5 * int64(time.Second)

//看板的数据更新时间
var DashboardUpdateTime = 3 * time.Second

//发送窗口的大小
var MaxSizeOfInflight int = 100

//sessionMgr处理新连接的chan缓冲大小
var MaxSizeOfNewConnectionChan int = 10000

//重发消息的最大次数
var MaxRetryTimes int = 5

//每个session 最大pedding消息数
var MaxSizeOfPublishMsg = 10 * 1024
