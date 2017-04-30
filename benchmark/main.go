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

func push(host string, i int) {
	//	mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker("tcp://" + host + ":1883").SetClientID(fmt.Sprintf("client%d", i))
	opts.SetKeepAlive(2 * time.Second)
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)

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
		time.Sleep(10 * time.Millisecond)
	}
}
func main() {
	host := flag.String("host", "10.168.37.53", "host")
	p := flag.Int("p", 1, "count process")
	flag.Parse()

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
	for i := 0; i < *p; i++ {
		go push(*host, i)
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
