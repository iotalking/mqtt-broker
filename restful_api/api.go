package api

import (
	"fmt"
	"net/http"

	"github.com/iotalking/mqtt-broker/session"
)

func Pub(w http.ResponseWriter, r *http.Request) {
	var topic string
	var msg string
	var qos string
	var q byte
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(401)
		return
	}
	topic = r.Form.Get("topic")
	msg = r.Form.Get("msg")
	qos = r.Form.Get("qos")
	if len(qos) > 0 {
		fmt.Sscan(qos, &q)
	}
	err = session.GetMgr().Publish(topic, []byte(msg), q)
	if err != nil {
		w.WriteHeader(403)
		return
	}
	w.WriteHeader(200)
}
