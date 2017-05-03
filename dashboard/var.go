package dashboard

import (
	"time"

	"github.com/iotalking/mqtt-broker/safe-runtine"
)

type OverviewData struct {
	//总运行时间(纳秒)
	RunNanoSeconds AtmI64

	RunTimeString string

	//总接入消息数
	RecvMsgCnt AtmI64

	//总接入字节
	RecvBytes AtmI64

	//总发送字节
	SentBytes AtmI64

	//总发送消息数
	SentMsgCnt AtmI64

	//每秒接入消息数
	RecvMsgPerSeconds AtmI64
	//每秒发送消息数
	SentMsgPerSeconds AtmI64

	//在线客户端
	ActiveClients AtmI64

	//未验证的客户端数
	InactiveClients AtmI64

	//最大在线客户端数
	MaxonLineClinets AtmI64

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

	runtine *runtine.SafeRuntine

	getChan chan byte
	outChan chan OverviewData
}

var Overview = &OverviewData{}

var startTime time.Time

func init() {
	startTime = time.Now()
}
