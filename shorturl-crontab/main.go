package main

import (
	"flag"
	"shorturl-crontab/cron"
	"shorturl-crontab/pkg/config"
	"shorturl-crontab/pkg/db/mysql"
	"shorturl-crontab/pkg/db/redis"
	"shorturl-crontab/pkg/log"
)

var (
	configFile = flag.String("config", "dev.config.yaml", "")
)

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

	// 初始化Redis连接池
	redis.InitRedisPool(cnf)

	cron.Run()
}
