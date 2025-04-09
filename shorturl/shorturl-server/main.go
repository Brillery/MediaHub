package main

import (
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"net"
	"shorturl/pkg/config"
	"shorturl/pkg/db/mysql"
	"shorturl/pkg/db/redis"
	"shorturl/pkg/log"
	"shorturl/proto"
	"shorturl/shorturl-server/cache"
	"shorturl/shorturl-server/data"
	"shorturl/shorturl-server/interceptor"
	"shorturl/shorturl-server/server"
)

var (
	configFile = flag.String("config", "dev.config.yaml", "")
)

// main 是应用程序的入口函数。它负责初始化配置、设置日志、创建数据库连接、初始化缓存、启动gRPC服务并监听指定端口。
func main() {
	flag.Parse()

	// 加载并解析指定的配置文件
	config.InitConfig(*configFile)
	cnf := config.GetConfig()

	// 配置日志设置，包括日志级别、输出路径和是否打印调用栈信息
	log.SetLevel(cnf.Log.Level)
	log.SetOutput(log.GetRotateWriter(cnf.Log.LogPath))
	log.SetPrintCaller(true)

	// 配置日志记录器的输出、日志级别和调用者打印设置
	logger := log.NewLogger()
	logger.SetOutput(log.GetRotateWriter(cnf.Log.LogPath))
	logger.SetLevel(cnf.Log.Level)
	logger.SetPrintCaller(true)

	// 初始化MySQL数据库连接池
	mysql.InitMysql(cnf)
	// 创建基于MySQL的URL映射数据工厂
	urlMapDataFactory := data.NewUrlMapDataFactory(logger, mysql.GetDB())

	// 初始化Redis连接池
	redis.InitRedisPool(cnf)
	// 创建基于Redis的键值缓存工厂
	kvCacheFactory := cache.NewRedisCacheFactory(redis.GetPool())

	// 绑定并监听指定的网络地址和端口
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cnf.Server.IP, cnf.Server.Port))
	if err != nil {
		log.Fatal(err)
	}

	// 创建gRPC服务器实例并注册ShortUrl服务
	s := grpc.NewServer(grpc.UnaryInterceptor(interceptor.UnaryAuthInterceptor), grpc.StreamInterceptor(interceptor.StreamAuthInterceptor))
	service := server.NewService(cnf, logger, urlMapDataFactory, kvCacheFactory)
	proto.RegisterShortUrlServer(s, service)

	// 多路复用健康检查
	grpc_health_v1.RegisterHealthServer(s, health.NewServer())

	// 启动gRPC服务并开始处理传入的请求
	if err = s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
