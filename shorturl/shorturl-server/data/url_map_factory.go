package data

import (
	"database/sql"
	"shorturl/pkg/constants"
	"shorturl/pkg/log"
)

// IUrlMapDataFactory 定义URL映射数据工厂接口
type IUrlMapDataFactory interface {
	NewUrlMapData(isPublic bool) IUrlMapData
}

type urlMapDataFactory struct {
	log log.ILogger
	db  *sql.DB
}

// NewUrlMapDataFactory 创建URL映射数据工厂实例
// 参数：
//
//	log：日志接口实例
//	db：数据库连接对象
//
// 返回：
//
//	实现IUrlMapDataFactory接口的工厂实例
func NewUrlMapDataFactory(log log.ILogger, db *sql.DB) IUrlMapDataFactory {
	return &urlMapDataFactory{
		log: log,
		db:  db,
	}
}

// NewUrlMapData 根据公开标识创建对应的URL映射数据对象
// 参数：
//
//	isPublic：布尔值，true表示使用公共表"url_map"，false使用用户表"url_map_user"
//
// 返回：
//
//	实现IUrlMapData接口的数据操作对象
func (f *urlMapDataFactory) NewUrlMapData(isPublic bool) IUrlMapData {
	tableName := constants.TABLENAME_URL_MAP
	// 根据公开标识切换数据表
	if !isPublic {
		tableName = constants.TABLENAME_URL_MAP_USER
	}
	return newUrlMapData(f.log, f.db, tableName)
}
