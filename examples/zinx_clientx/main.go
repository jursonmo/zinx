package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aceld/zinx/clientx"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

func pingLoop(conn ziface.IConnection) {
	for {
		data := strings.Repeat("a", 10)
		//err := conn.SendMsg(1, []byte("Ping...Ping...Ping...[FromClient]"))
		err := conn.SendMsg(1, []byte(data))
		if err != nil {
			zlog.Error(err)
			break
		}

		time.Sleep(2 * time.Second)
		err = conn.SendMsg(1, []byte(data))
		if err != nil {
			zlog.Error(err)
			break
		}
	}
	zlog.Error("pingLoop exit")
}

// Executed when a connection is created
func onClientStart(conn ziface.IConnection) {
	fmt.Println("onClientStart is Called ... ")
	go pingLoop(conn)
}

func onClientStop(conn ziface.IConnection) {
	fmt.Println("onClientStop is Called ... ")
}

func main() {
	go startServer()
	time.Sleep(time.Millisecond * 10)

	// StartWithContext() 示例
	ExampleStartWithContext()

	time.Sleep(time.Second * 2)
	// Connect() 示例
	ExampleConnect()

	time.Sleep(time.Second * 2)
	// ConnectWithTimeout() 示例
	ExampleConnectWithTimeout()

	select {}
}

func ExampleStartWithContext() {
	client := clientx.NewClient("127.0.0.1", 8999)
	client.SetOnConnStart(onClientStart)
	client.SetOnConnStop(onClientStop)

	client.StartWithContext(context.Background())

	time.Sleep(time.Second * 2)
	client.Stop()
}

func ExampleConnect() {
	client := clientx.NewClient("127.0.0.1", 8999)
	client.SetOnConnStart(onClientStart)
	client.SetOnConnStop(onClientStop)

	err := client.Connect(context.Background())
	if err != nil {
		zlog.Error("connect failed, err:", err)
		return
	}
	zlog.Info("connect success")

	time.Sleep(time.Second * 2)
	client.Stop()
}

func ExampleConnectWithTimeout() {
	client := clientx.NewClient("127.0.0.1", 8999)
	client.SetOnConnStart(onClientStart)
	client.SetOnConnStop(onClientStop)

	err := client.ConnectWithTimeout(5 * time.Second)
	if err != nil {
		zlog.Error("connect failed, err:", err)
		return
	}
	zlog.Info("connect success")

	time.Sleep(time.Second * 2)
	client.Stop()
}

type PingRouter struct {
	znet.BaseRouter
}

// Ping Handle MsgId=1
func (r *PingRouter) Handle(request ziface.IRequest) {
	//read client data
	fmt.Println("recv from client : msgId=", request.GetMsgID(), "len=", len(request.GetData()), "data=", string(request.GetData()))
}

func startServer() {
	s := znet.NewServer()

	//2 configure routing
	s.AddRouter(1, &PingRouter{})

	//3 start service
	s.Serve()
}
