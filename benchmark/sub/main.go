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
	"github.com/iotalking/mqtt-broker/utils"
)

var host = flag.String("h", "localhost", "mqtt server address.protocal://ip:port,protocal:mqtt,mqtts,ws,wss")
var topic = flag.String("t", "", "topic")
var qos = flag.Int("q", 0, "the QoS of the message")
var port = flag.Int("p", 1883, "the server port")
var loglevel = flag.String("log", "error", "set log level.")
var id = flag.String("i", "", "the id of the client")

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
	}
	if len(*id) == 0 {
		*id = utils.NewId()
	}
	c := client.NewClient(*id)
	token, err := c.Connect(protocal, fmt.Sprintf("%s:%d", *host, *port))
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
	c.Subcribe(map[string]byte{
		*topic: byte(*qos),
	})

	signal.Notify(signals, os.Interrupt)

	select {
	case <-signals:
		c.Disconnect()
	}
}

func pubmqtt() {

}
