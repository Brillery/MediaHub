package mysql

// 导入必要的包
import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"shorturl-crontab/pkg/config"
	"time"
)

// db 用于存储全局的数据库连接实例
var db *sql.DB

// InitMysql 初始化MySQL数据库连接
// 参数 cnf 包含了数据库连接所需的配置信息
func InitMysql(cnf *config.Config) {
	// 检查数据库连接字符串是否为空，如果为空则抛出panic
	if cnf.Mysql.DSN == "" { // dsn 数据源名称
		panic("数据库连接字符串不能为空")
	}
	// 使用提供的连接字符串打开数据库连接
	var err error
	db, err = sql.Open("mysql", cnf.Mysql.DSN)
	if err != nil {
		panic(err)
	}
	// 设置数据库连接的各种配置参数
	db.SetMaxOpenConns(cnf.Mysql.MaxOpenConn)
	db.SetMaxIdleConns(cnf.Mysql.MaxIdleConn)
	db.SetConnMaxLifetime(time.Second * time.Duration(cnf.Mysql.MaxLifeTime))
}

// GetDB 返回全局的数据库连接实例
func GetDB() *sql.DB {
	return db
}
