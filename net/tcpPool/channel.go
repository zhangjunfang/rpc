package tcpPool

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

// channelPool 实现Pool接口 并且带有缓冲的连接池.
type channelPool struct {
	//mu 为了保证每个连接获取是协成安全的
	mu sync.Mutex
	//连接的缓存
	conns chan net.Conn

	// 创建新连接的工厂方法
	factory Factory
}

// Factory 获取创建一个连接
type Factory func() (net.Conn, error)

func NewChannelPool(initialCap, maxCap int, factory Factory) (Pool, error) {

	if initialCap < 0 || maxCap <= 0 || initialCap > maxCap {

		return nil, errors.New("invalid capacity settings")

	}

	c := &channelPool{
		conns:   make(chan net.Conn, maxCap),
		factory: factory,
	}

	for i := 0; i < initialCap; i++ {
		conn, err := factory()
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		c.conns <- conn
	}

	return c, nil
}

func (c *channelPool) getConns() chan net.Conn {

	c.mu.Lock()

	conns := c.conns

	c.mu.Unlock()

	return conns
}

func (c *channelPool) wrapConn(conn net.Conn) net.Conn {
	p := &PoolConn{c: c}
	p.Conn = conn
	return p
}

func (c *channelPool) Get() (net.Conn, error) {
	conns := c.getConns()

	if conns == nil {

		return nil, ErrClosed

	}

	select {

	case conn := <-conns:

		if conn == nil {

			return nil, ErrClosed

		}

		return c.wrapConn(conn), nil

	default:

		conn, err := c.factory()

		if err != nil {

			return nil, err

		}

		return c.wrapConn(conn), nil
	}
}
func (c *channelPool) put(conn net.Conn) error {

	if conn == nil {

		return errors.New("connection is nil. rejecting")

	}

	c.mu.Lock()

	defer c.mu.Unlock()

	if c.conns == nil {
		return conn.Close()
	}

	select {

	case c.conns <- conn:

		return nil

	default:

		return conn.Close()

	}
}

func (c *channelPool) Close() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.factory = nil
	c.mu.Unlock()

	if conns == nil {
		return
	}

	close(conns)

	for conn := range conns {
		conn.Close()
	}
}

func (c *channelPool) Len() int {
	return len(c.getConns())
}
