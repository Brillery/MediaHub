package main

import (
	"enterprise-project1-mediahub/mediahub/controller"
	"enterprise-project1-mediahub/mediahub/middleware"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/storage/cos"
	"enterprise-project1-mediahub/mediahub/routers"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
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

	// 创建COS存储工厂实例，使用配置中的存储参数
	sf := cos.NewCosStorageFactory(cnf.Cos.BucketUrl, cnf.Cos.SecretId, cnf.Cos.SecretKey, cnf.Cos.CDNDomain)

	// 初始化控制器，传入存储工厂、日志记录器和全局配置
	controller := controller.NewController(sf, logger, cnf)

	// 设置Gin运行模式并创建路由分组
	gin.SetMode(cnf.Http.Mode)
	r := gin.Default()
	r.Use(middleware.Cors(), middleware.Auth())
	// 这里是一次最简单的健康检查，后续可以进行健康检查的完善
	r.GET("/health", func(*gin.Context) {})
	api := r.Group("/api")

	// 初始化API路由并绑定控制器
	routers.InitRouters(api, controller)

	// 配置静态文件服务和默认路由处理
	fs := http.FileServer(http.Dir("www"))
	r.NoRoute(func(ctx *gin.Context) {
		fs.ServeHTTP(ctx.Writer, ctx.Request)
	})
	r.GET("/", func(ctx *gin.Context) {
		http.ServeFile(ctx.Writer, ctx.Request, "www/index.html")
	})

	// 启动HTTP服务，监听指定IP和端口
	err := r.Run(fmt.Sprintf("%s:%d", cnf.Http.IP, cnf.Http.Port))
	if err != nil {
	}
	log.Fatal(err)
}
