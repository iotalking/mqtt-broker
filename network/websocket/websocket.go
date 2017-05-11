package websocket

import (
	"io"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/iotalking/mqtt-broker/session"

	log "github.com/Sirupsen/logrus"
)

var mux = http.NewServeMux()
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"mqtt", "mqttv3.1"},
}

func Start(addr string) {
	log.Infof("starting websocket on :%s", addr)
	mux.HandleFunc("/mqtt", serverWS)
	s := http.Server{}
	s.Addr = addr
	s.Handler = mux
	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

type readData struct {
	buf []byte
	n   int
	err error
}
type writeData struct {
	buf []byte
	n   int
	err error
}
type wsReadWriteCloser struct {
	reader io.Reader
	conn   *websocket.Conn
}

func (this *wsReadWriteCloser) Read(p []byte) (n int, err error) {

next:
	if this.reader == nil {
		_, this.reader, err = this.conn.NextReader()
		if err != nil {
			return
		}
	}
	n, err = this.reader.Read(p)
	if err == io.EOF {
		this.reader = nil
		goto next
	}
	return
}
func (this *wsReadWriteCloser) Write(p []byte) (int, error) {
	writer, err := this.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer writer.Close()
	return writer.Write(p)
}
func (this *wsReadWriteCloser) Close() error {
	err := this.conn.Close()
	return err
}

func serverWS(resp http.ResponseWriter, req *http.Request) {
	log.Debug("serverWS comming")
	resp.Header().Add("Access-Control-Allow-Origin", "*")
	swp := req.Header.Get("Sec-Websocket-Protocol")
	log.Debug("Sec-Websocket-Protocol:", swp)

	conn, err := upgrader.Upgrade(resp, req, nil)
	//	_, err := upgrader.Upgrade(resp, req, nil)
	if err != nil {
		log.Errorf("serverWS error:%s", err)
		return
	}

	sessionMgr := session.GetMgr()
	rwc := wsReadWriteCloser{
		conn: conn,
	}
	sessionMgr.HandleConnection(&rwc)
	log.Debug("ws mqtt exited")
}
