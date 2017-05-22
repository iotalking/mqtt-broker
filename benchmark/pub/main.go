package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/iotalking/mqtt-broker/client"
	"github.com/iotalking/mqtt-broker/session"
	"github.com/iotalking/mqtt-broker/utils"
)

var host = flag.String("h", "mqtt://localhost:1883", "mqtt server address.protocal://[username][:password]@ip[:port],protocal:mqtt,mqtts,ws,wss")
var topic = flag.String("t", "", "topic")
var qos = flag.Int("q", 0, "the QoS of the message")
var body = flag.String("m", "", "the message body")
var loglevel = flag.String("log", "error", "set log level.")
var id = flag.String("i", "", "the id of the client")
var times = flag.Int("c", 1, "publish message times")
var retain = flag.Bool("r", false, "retain message")

func main() {
	log.SetOutput(os.Stdout)

	flag.Parse()

	level, err := log.ParseLevel(*loglevel)
	if err != nil {
		flag.Usage()
		return
	}
	log.SetLevel(level)
	if len(*host) <= 0 {
		flag.Usage()
		return
	}
	if len(*topic) == 0 {
		*topic = flag.Arg(0)
	}
	if len(*topic) == 0 {
		flag.Usage()
		return
	}
	go http.ListenAndServe(":6061", nil)

	var protocal = "tcp"
	t := strings.Split(*host, "://")
	if len(t) < 2 {
		protocal = "mqtt"
	} else {
		protocal = t[0]
		*host = t[1]
	}
	if len(*id) == 0 {
		*id = utils.NewId()
	}
	c := client.NewClient(*id, session.GetMgr())
	token, err := c.Connect(protocal, *host)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		return
	}
	token.Wait()
	c.SetOnMessage(func(topic string, body []byte, qos byte) {
		fmt.Printf("qos:%d topic:%s payload:%s\n", qos, topic, body)
	})
	signals := make(chan os.Signal, 1)
	c.SetOnDisconnected(func() {
		log.Debug("client disconnected")
		signals <- os.Interrupt
	})
	go func() {
		for *times > 0 {
			token, err := c.Publish(*topic, []byte(*body), byte(*qos), *retain)
			if err != nil {
				log.Error("publish error:", err)
				break
			}
			token.Wait()
			*times--
		}
		signals <- os.Interrupt
	}()

	signal.Notify(signals, os.Interrupt)

	select {
	case <-signals:
		c.Disconnect()
	}
}
