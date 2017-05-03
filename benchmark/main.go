/*
 * Copyright (c) 2013 IBM Corp.
 *
 * All rights reserved. This program and the accompanying materials
 * are made available under the terms of the Eclipse Public License v1.0
 * which accompanies this distribution, and is available at
 * http://www.eclipse.org/legal/epl-v10.html
 *
 * Contributors:
 *    Seth Hoenig
 *    Allan Stockdill-Mander
 *    Mike Robertson
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	//	fmt.Printf("TOPIC: %s\n", msg.Topic())
	//	fmt.Printf("MSG: %s\n", msg.Payload())
}

var clients []mqtt.Client
var newClientChan = make(chan mqtt.Client)
var closeAll = make(chan bool)

func push(host string, port int, i int) {
	//	mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker("tcp://" + host + fmt.Sprintf(":%d", port)).SetClientID(fmt.Sprintf("client%d", i))
	opts.SetKeepAlive(20 * time.Second)
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(10 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := c.Subscribe("go-mqtt/sample", 0, nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	for i := 0; true; i++ {
		text := fmt.Sprintf("this is msg #%d!", i)
		token := c.Publish("go-mqtt/sample", 0, false, text)
		token.Wait()
		time.Sleep(time.Second)
	}
}
func subscribe(host string, port int, i int, resultCh chan<- bool) {
	//	mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker("tcp://" + host + fmt.Sprintf(":%d", port)).SetClientID(fmt.Sprintf("client%d", time.Now().Nanosecond()))
	opts.SetConnectTimeout(100 * time.Second)

	opts.SetKeepAlive(200 * time.Second)
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(100 * time.Second)

	defer func() {
		recover()
		close(resultCh)
	}()
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := c.Subscribe("go-mqtt/sample", 0, nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

}
func main() {
	host := flag.String("host", "localhost", "host")
	port := flag.Int("port", 1883, "server port")
	p := flag.Int("p", 1, "count process")
	flag.Parse()
	var i int
	defer func() {
		recover()
		mqtt.DEBUG.Printf("connected :%d", i)
	}()
	go func() {
		for {
			select {
			case c := <-newClientChan:
				clients = append(clients, c)
			case <-closeAll:
				for _, c := range clients {
					c.Disconnect(250)
				}
				close(closeAll)
				return
			}
		}
	}()
	//	go push(*host,*port, 0)
	for i = 1; i <= *p; i++ {
		ch := make(chan bool)
		go subscribe(*host, *port, i, ch)
		<-ch
	}
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt)
	select {
	case <-signals:
		//关闭服务器的端口监听，以退出
		closeAll <- true
	}
	<-closeAll
}
