package cron

import (
	"context"
	"github.com/robfig/cron/v3"
	"shorturl-crontab/data"
	"shorturl-crontab/pkg/db/mysql"
	"shorturl-crontab/pkg/db/redis"
	"shorturl-crontab/pkg/log"
	"time"
)

const DefaultUrlMapTTL = 30 * 86400 // 默认URL映射缓存有效期30天（单位：秒）

// Run 启动定时任务，每天凌晨3点执行setUrlMapID函数，并立即执行一次初始化。
// 该函数负责初始化和运行cron调度器。
func Run() {
	setUrlMapID() // 初始化时立即执行一次
	c := cron.New()
	c.AddFunc("0 3 * * *", setUrlMapID) // 每日3点执行定时任务
	c.Run()
}

// setUrlMapID 从MySQL数据库获取url_map和url_map_user表的最大ID值，并将其缓存到Redis中
func setUrlMapID() {
	tables := []string{"url_map", "url_map_user"} // 需要同步的数据库表名列表

	// 获取Redis连接池及客户端
	redisPool := redis.GetPool()
	client := redisPool.Get()
	defer redisPool.Put(client)

	db := data.NewData(mysql.GetDB()) // 创建数据库操作对象
	for _, t := range tables {
		// 获取当前表的最大ID值
		id, err := db.GetMaxID(t)
		if err != nil {
			log.Error(err) // 记录错误信息
			continue       // 跳过当前表处理
		}

		key := redis.GetKey(t, "max_id") // 生成Redis键名
		// 设置Redis键值对并设置过期时间
		err = client.SetEx(context.Background(), key, id, time.Second*time.Duration(DefaultUrlMapTTL)).Err()
		if err != nil {
			log.Error(err) // 记录Redis设置失败错误
			continue       // 继续处理下一个表
		}
	}
}
