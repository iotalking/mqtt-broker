package session

type SessionInfo struct {
	Id string
	//等待发送的消息数
	PeddingMsgCnt int
	//正在发送中的消息数
	InflightMsgCnt int
	//在Channel发送队列中的包数
	SendingMsgCnt int
}
