package grpc_client_pool

import (
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"sync"
)

type ClientPool interface {
	Get() *grpc.ClientConn
	Put(conn *grpc.ClientConn)
}

type clientPool struct {
	pool sync.Pool
}

func NewPool(target string, opts ...grpc.DialOption) (ClientPool, error) {
	return &clientPool{
		pool: sync.Pool{
			New: func() any {
				conn, err := grpc.Dial(target, opts...)
				if err != nil {
					log.Error(err)
					return nil
				}
				return conn
			},
		},
	}, nil
}

func (c *clientPool) Get() *grpc.ClientConn {
	conn := c.pool.Get().(*grpc.ClientConn)
	if conn.GetState() == connectivity.Shutdown || conn.GetState() == connectivity.TransientFailure {
		conn = c.pool.Get().(*grpc.ClientConn) // 为什么要类型推断，因为pool源码里返回的是一个any类型
	}
	return conn
}

func (c *clientPool) Put(conn *grpc.ClientConn) {
	if conn.GetState() == connectivity.Shutdown || conn.GetState() == connectivity.TransientFailure {
		conn.Close()
		return
	}
	c.pool.Put(conn)
}
