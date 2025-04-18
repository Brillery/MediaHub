package grpc_client_pool

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"log"
	"sync"
)

type ClientPool interface {
	Get() *grpc.ClientConn
	Put(conn *grpc.ClientConn)
}

type clientCusPool struct {
	mutex      sync.Mutex
	conns      []*grpc.ClientConn
	maxConnNum int
	target     string
	opts       []grpc.DialOption
	currIndex  int
}

func NewClientCusPool(target string, maxConnNum int, opts ...grpc.DialOption) (ClientPool, error) {
	if maxConnNum <= 0 {
		maxConnNum = 1
	}
	return &clientCusPool{
		mutex:      sync.Mutex{},
		conns:      make([]*grpc.ClientConn, maxConnNum),
		maxConnNum: maxConnNum,
		target:     target,
		opts:       opts,
		currIndex:  0,
	}, nil
}

func (c *clientCusPool) new() *grpc.ClientConn {
	conn, err := grpc.Dial(c.target, c.opts...)
	if err != nil {
		log.Println(err)
		return nil
	}
	return conn
}
func (c *clientCusPool) Get() *grpc.ClientConn {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.currIndex += 1
	if c.currIndex >= c.maxConnNum {
		c.currIndex = 0
	}
	conn := c.conns[c.currIndex]
	if conn == nil || conn.GetState() == connectivity.Shutdown || conn.GetState() == connectivity.TransientFailure {
		if conn != nil {
			conn.Close()
		}
		conn = c.new()
		c.conns[c.currIndex] = conn
	}
	return conn
}

func (c *clientCusPool) Put(conn *grpc.ClientConn) {
	if conn.GetState() == connectivity.Shutdown || conn.GetState() == connectivity.TransientFailure {
		conn.Close()
	}
}

// package grpc_client_pool
//
//import (
//	"google.golang.org/grpc"
//	"google.golang.org/grpc/connectivity"
//	"log"
//	"sync"
//)
//
//type ClientPool interface {
//	Get() *grpc.ClientConn
//	Put(conn *grpc.ClientConn)
//}
//
//type clientCusPool struct {
//	mutex      sync.Mutex
//	conns      []*grpc.ClientConn
//	maxConnNum int
//	target     string
//	opts       []grpc.DialOption
//	currIndex  int
//}
//
//func NewClientCusPool(target string, maxConnNum int, opts ...grpc.DialOption) (ClientPool, error) {
//	if maxConnNum <= 0 {
//		maxConnNum = 1
//	}
//	return &clientCusPool{
//		mutex:      sync.Mutex{},
//		conns:      make([]*grpc.ClientConn, maxConnNum),
//		maxConnNum: maxConnNum,
//		target:     target,
//		opts:       opts,
//		currIndex:  0,
//	}, nil
//}
//
//func (c *clientCusPool) new() *grpc.ClientConn {
//	conn, err := grpc.Dial(c.target, c.opts...)
//	if err != nil {
//		log.Println(err)
//		return nil
//	}
//	return conn
//}
//func (c *clientCusPool) Get() *grpc.ClientConn {
//	c.mutex.Lock()
//	defer c.mutex.Unlock()
//	c.currIndex += 1
//	if c.currIndex >= c.maxConnNum {
//		c.currIndex = 0
//	}
//	conn := c.conns[c.currIndex]
//	if conn == nil || conn.GetState() == connectivity.Shutdown || conn.GetState() == connectivity.TransientFailure {
//		if conn != nil {
//			conn.Close()
//		}
//		conn = c.new()
//		c.conns[c.currIndex] = conn
//	}
//	return conn
//}
//
//func (c *clientCusPool) Put(conn *grpc.ClientConn) {
//	if conn.GetState() == connectivity.Shutdown || conn.GetState() == connectivity.TransientFailure {
//		conn.Close()
//	}
//}
