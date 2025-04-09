package main

import (
	"enterprise-project1-mediahub/shorturl-proxy/pkg/config"
	"enterprise-project1-mediahub/shorturl-proxy/pkg/log"
	"enterprise-project1-mediahub/shorturl-proxy/proxy"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
)

var configFile = flag.String("config", "dev.config.yaml", "")

func main() {
	// 解析命令行参数
	flag.Parse()

	// 初始化配置信息
	config.InitConfig(*configFile)
	cnf := config.GetConfig()

	// 配置日志系统，设置级别、输出到指定路径及打印调用者信息
	log.SetLevel(cnf.Log.Level)
	log.SetOutput(log.GetRotateWriter(cnf.Log.LogPath))
	log.SetPrintCaller(true)

	// 创建自定义日志记录器并设置相同的日志配置
	logger := log.NewLogger()
	logger.SetOutput(log.GetRotateWriter(cnf.Log.LogPath))
	logger.SetLevel(cnf.Log.Level)
	logger.SetPrintCaller(true)

	// 设置Gin运行模式并创建路由分组
	gin.SetMode(cnf.Http.Mode)
	r := gin.Default()
	// 这里是一次最简单的健康检查，后续可以进行健康检查的完善
	r.GET("/health", func(*gin.Context) {})

	p := proxy.NewProxy(cnf, logger)
	public := r.Group("/p")
	public.GET("/:short_key", p.PublicProxy)
	user := r.Group("/u")
	user.GET("/:short_key", p.UserProxy)

	// 启动HTTP服务，监听指定IP和端口
	r.Run(fmt.Sprintf("%s:%d", cnf.Http.IP, cnf.Http.Port))
}
