package dashboard

import (
	"github.com/iotalking/mqtt-broker/utils"
)

type OverviewData struct {
	//总运行时间(纳秒)
	RunNanoSeconds AtmI64
	RunTimeString  string

	//服务器的唯一标识
	BrokerId string
	//服务器的网络地址
	BrokerAddress string

	//总接入消息数
	RecvMsgCnt AtmI64
	//总发送消息数
	SentMsgCnt AtmI64

	//总接入字节
	RecvBytes AtmI64

	//总发送字节
	SentBytes AtmI64

	//每秒接入消息数
	RecvBytesPerSeconds AtmI64
	//每秒发送消息数
	SentBytesPerSeconds AtmI64
	//每秒接入消息数
	RecvMsgPerSeconds AtmI64
	//每秒发送消息数
	SentMsgPerSeconds AtmI64

	//在线客户端
	ActiveClients AtmI64

	//未验证的客户端数
	InactiveClients AtmI64

	//最大在线客户端数
	MaxActiveClinets AtmI64

	//总主题数
	TipicCnt AtmI64

	//总订阅数
	Subscritions AtmI64

	//保存的总消息数
	RetainedMsgCnt AtmI64

	//未发送消息数
	//由于session端的发达缓冲满而丢的包
	DropMsgCnt AtmI64

	//未处理消息数
	//从网络接入到，但由于接入缓冲满而求能处理的包数
	UnrecvMsgCnt AtmI64

	//消息缓冲峰值
	PeekPublishBufferCnt AtmI64

	//消息缓冲当前值
	CurPUblishBufferCnt AtmI64

	//发送超时的客户端数
	SentErrClientCnt AtmI64

	//ticker map中的ticker总数
	TickerMapCnt AtmI64
	//添加的ticker总数
	AddTickerCnt AtmI64

	//打开的文件句柄数
	OpenedFiles AtmI64
	//打开文件峰值
	MaxOpenedFiles AtmI64

	//正在关闭的文件句柄数
	ClosingFiles AtmI64

	//各类消息数
	ConnectSentCnt AtmI64
	ConnackSentCnt AtmI64

	PublishSentCnt AtmI64
	PubackRecvCnt  AtmI64

	PubrecRecvCnt  AtmI64
	PubrelSentCnt  AtmI64
	PubcompRecvCnt AtmI64

	SubscribeSentCnt   AtmI64
	SubackSentCnt      AtmI64
	UnsubscribeSentCnt AtmI64
	UnsubackSentCnt    AtmI64
	PingreqSentCnt     AtmI64
	PingrespSentCnt    AtmI64
	DisconectSentCnt   AtmI64

	ConnectRecvCnt AtmI64
	ConnackRecvCnt AtmI64

	PublishRecvCnt AtmI64
	PubackSentCnt  AtmI64

	PubrecSentCnt  AtmI64
	PubrelRecvCnt  AtmI64
	PubcompSentCnt AtmI64

	SubscribeRecvCnt   AtmI64
	SubackRecvCnt      AtmI64
	UnsubscribeRecvCnt AtmI64
	UnsubackRecvCnt    AtmI64
	PingreqRecvCnt     AtmI64
	PingrespRecvCnt    AtmI64
	DisconectRecvCnt   AtmI64

	//Ticker最后一个耗时(ns）
	LastTickerBusyTime AtmI64
	//定局定时器轮询最大耗时(ns）
	MaxTickerBusyTime AtmI64
}

var Overview = &OverviewData{}

func init() {
	Overview.BrokerId = utils.LocalId()
}
