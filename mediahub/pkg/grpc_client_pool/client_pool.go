// Package grpc_client_pool 提供 mediahub 调用内部 gRPC 服务时使用的固定容量连接池。
//
// 模块职责：
// - 输入：目标 gRPC 地址、DialOption 以及业务请求时的 Get/Put 调用。
// - 输出：可复用的 *grpc.ClientConn，减少每次上传生成短链时重复建连。
// - 状态边界：只维护当前进程内的连接切片和轮询游标，不维护业务请求状态。
// - 外部依赖：google.golang.org/grpc 连接状态，不负责服务发现、鉴权 token 或重试策略。
// - 并发边界：Get/Put 通过互斥锁保护连接槽位，适用于单实例进程内复用；跨实例连接数仍由各实例独立维护。
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
