package clientx

import (
	"context"
	"testing"
	"time"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/stretchr/testify/assert"
)

func mockServer(host string, port int) ziface.IServer {
	zsconf := *zconf.GlobalObject
	zsconf.Host = host
	zsconf.TCPPort = port

	s := znet.NewUserConfServer(&zsconf)
	go s.Serve()
	// 等待服务器启动完成
	time.Sleep(100 * time.Millisecond)
	return s
}

func TestNewClient(t *testing.T) {
	client := NewClient("localhost", 8080)
	assert.NotNil(t, client)
	assert.NotNil(t, client.IClient)
	assert.NotNil(t, client.disconnectCh)
	assert.NotNil(t, client.connectOkCh)
	assert.Equal(t, time.Second, client.reconnectIntvl)
}

func TestSetReconnectIntvl(t *testing.T) {
	client := NewClient("localhost", 8080)
	newInterval := 5 * time.Second
	client.SetReconnectIntvl(newInterval)
	assert.Equal(t, newInterval, client.reconnectIntvl)
}

func TestStartWithContext(t *testing.T) {
	server := mockServer("localhost", 8080)

	client := NewClient("localhost", 8080)
	client.SetReconnectIntvl(50 * time.Millisecond)
	var onConnStartCalled, onConnStopCalled bool
	client.SetOnConnStart(func(conn ziface.IConnection) {
		onConnStartCalled = true
	})
	client.SetOnConnStop(func(conn ziface.IConnection) {
		onConnStopCalled = true
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.StartWithContext(ctx)

	// 休眠100ms, 确保客户端已连接
	time.Sleep(100 * time.Millisecond)
	assert.True(t, client.IsConnected())
	assert.True(t, onConnStartCalled) // 确保onConnStart回调被调用

	// 停止服务器, 测试客户端是否会断开连接
	server.Stop()
	time.Sleep(100 * time.Millisecond)
	assert.False(t, client.IsConnected())
	assert.True(t, onConnStopCalled) // 确保onConnStop回调被调用

	// 再次启动服务器，测试客户端是否重新连接
	server = mockServer("localhost", 8080)
	defer server.Stop()

	time.Sleep(time.Millisecond * 200)
	assert.True(t, client.IsConnected())

}

func TestConnectWithTimeout(t *testing.T) {
	// 测试连接超时
	client := NewClient("localhost", 8080)
	err := client.ConnectWithTimeout(100 * time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connect timeout")

	// 启动服务器, 测试client连接成功
	server := mockServer("localhost", 8081)
	defer server.Stop()
	client = NewClient("localhost", 8081)
	err = client.ConnectWithTimeout(time.Second)
	assert.NoError(t, err)

}

func TestConnect(t *testing.T) {
	server := mockServer("localhost", 8082)
	defer server.Stop()
	client := NewClient("localhost", 8082)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 测试连接成功, err应该为nil
	err := client.Connect(ctx)
	assert.NoError(t, err)

	// 测试取消连接
	client = NewClient("localhost", 8083) // 使用一个不存在的端口
	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	err = client.Connect(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestStop(t *testing.T) {
	server := mockServer("localhost", 8085)
	defer server.Stop()

	client := NewClient("localhost", 8085)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.StartWithContext(ctx)

	// 确保客户端已连接
	time.Sleep(100 * time.Millisecond)
	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	client.Stop()
	time.Sleep(100 * time.Millisecond)
	// 验证客户端已经断开
	if client.IsConnected() {
		t.Error("Client should not be connected")
	}
}
