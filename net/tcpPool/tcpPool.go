package tcpPool

import (
	"errors"
	"net"
)

const ()

var (
	// ErrClosed 表示连接池已经关闭错误.
	ErrClosed = errors.New("pool is closed")
)

func init() {

}

//连接池基本功能描述。一个连接池应该有最大，最小容量。设计合理的连接池应该是线程安全并且容易使用。
type Pool interface {
	Get() (net.Conn, error)
	Close()
	Len() int
}
