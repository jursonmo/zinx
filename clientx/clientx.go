package clientx

//clientx是对Zinx客户端进行封装了,实现异步连接和同步连接的功能以及自动重连功能。
// clientx is a wrapper for the Zinx client, implementing asynchronous and synchronous connection functionality as well as automatic reconnection.
import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

// Client 结构体封装了 Zinx 客户端，提供了额外的连接管理功能
// Client struct encapsulates the Zinx client, providing additional connection management features
type Client struct {
	ziface.IClient
	ctx            context.Context
	cancel         context.CancelFunc
	disconnectCh   chan struct{}
	connectOkCh    chan struct{}
	connected      atomic.Value
	reconnectIntvl time.Duration
}

// NewClient 创建一个新的客户端实例, 参数保持与znet.NewClient一致
// NewClient creates a new client instance, with parameters consistent with znet.NewClient
func NewClient(host string, port int, opts ...znet.ClientOption) *Client {
	c := &Client{
		disconnectCh:   make(chan struct{}, 1),
		connectOkCh:    make(chan struct{}, 1),
		reconnectIntvl: time.Second,
	}
	c.connected.Store(false)
	c.IClient = znet.NewClient(host, port, opts...)

	c.IClient.SetOnConnStart(c.onConnStart) //默认设置连接成功的回调是往connectOkCh中发送消息
	c.IClient.SetOnConnStop(c.onConnStop)   //默认设置断开连接的回调是往disconnectCh中发送消息
	return c
}

// SetReconnectIntvl 设置重连间隔时间
// SetReconnectIntvl sets the reconnection interval time
func (c *Client) SetReconnectIntvl(intvl time.Duration) {
	c.reconnectIntvl = intvl
}

// SetOnConnStart 设置连接成功时的回调函数，同时保留默认的通知行为
// SetOnConnStart sets the callback function for successful connection while preserving the default notification behavior
func (c *Client) SetOnConnStart(handler func(conn ziface.IConnection)) {
	handlerWrap := func(conn ziface.IConnection) {
		c.onConnStart(conn)
		handler(conn)
	}
	c.IClient.SetOnConnStart(handlerWrap)
}

// onConnStart 内部方法，在连接成功时通知
// onConnStart internal method, notifies when connection is successful
func (c *Client) onConnStart(conn ziface.IConnection) {
	c.connected.Store(true)
	select {
	case c.connectOkCh <- struct{}{}:
	default:
	}
}

func (c *Client) clearConnectOkCh() {
	select {
	case <-c.connectOkCh:
	default:
	}
}

// SetOnConnStop 设置连接断开时的回调函数，同时保留默认的通知行为
// SetOnConnStop sets the callback function for connection termination while preserving the default notification behavior
func (c *Client) SetOnConnStop(handler func(conn ziface.IConnection)) {
	handlerWrap := func(conn ziface.IConnection) {
		c.onConnStop(conn)
		handler(conn)
	}
	c.IClient.SetOnConnStop(handlerWrap)
}

// onConnStop 内部方法，在连接断开时通知
// onConnStop internal method, notifies when connection is terminated
func (c *Client) onConnStop(conn ziface.IConnection) {
	c.connected.Store(false)
	select {
	case c.disconnectCh <- struct{}{}:
	default:
	}
}

// Stop 通过取消上下文来停止客户端，
// Stop stops the client by canceling the context
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Client) IsConnected() bool {
	return c.connected.Load().(bool)
}

// StartWithContext 使用给定的上下文启动客户端，并管理重连逻辑
// StartWithContext starts the client with the given context and manages reconnection logic
func (c *Client) StartWithContext(ctx context.Context) {
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.IClient.Start()

	go func() {
		for {
			select {
			case <-c.ctx.Done():
				if c.Conn() != nil {
					c.IClient.Stop()
				}
				return
			case <-c.disconnectCh:
				zlog.Errorf("%v->%v, disconnect, reconnect after %v", c.Conn().LocalAddr(), c.Conn().RemoteAddr(), c.reconnectIntvl)
				c.clearConnectOkCh() //清除connectOkCh中的消息，避免connectOkCh残留消息导致误认为连接成功
				time.Sleep(c.reconnectIntvl)
				c.IClient.Restart()
			case err := <-c.GetErrChan():
				zlog.Errorf("dial err:%v, reconnect after %v", err, c.reconnectIntvl)
				time.Sleep(c.reconnectIntvl)
				c.IClient.Restart()
			}
		}
	}()
}

// Connect 同步连接服务器，直到连接成功或者取消连接
// Connect synchronously connects to the server until the connection is successful or cancelled
func (c *Client) Connect(ctx context.Context) error {
	c.StartWithContext(ctx)
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	case <-c.connectOkCh:
		return nil
	}
}

// ConnectWithTimeout 同步连接服务器，直到连接成功或者连接超时
// ConnectWithTimeout synchronously connects to the server until the connection is successful or times out
func (c *Client) ConnectWithTimeout(timeout time.Duration) error {
	t := time.NewTimer(timeout)
	defer t.Stop()

	c.StartWithContext(context.Background())
	select {
	case <-t.C:
		c.Stop()
		return errors.New("connect timeout")
	case <-c.connectOkCh:
		return nil
	}
}
