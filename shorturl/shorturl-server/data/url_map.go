package data

import (
	"database/sql"
	"fmt"
	"github.com/pkg/errors"
	"shorturl/pkg/log"
	"shorturl/pkg/zerror"
)

// UrlMapEntity 表示URL映射的实体对象，包含原始URL及其短链接映射关系
type UrlMapEntity struct {
	ID          int64  `json:"id"`           // 主键ID
	UserID      int64  `json:"user_id"`      // 用户ID（0表示公共链接）
	ShortKey    string `json:"short_key"`    // 短链接键
	OriginalUrl string `json:"original_url"` // 原始URL
	Times       int    `json:"times"`        // 访问次数
	CreateAt    int64  `json:"create_at"`    // 创建时间戳
	UpdateAt    int64  `json:"update_at"`    // 最后更新时间戳
}

// IUrlMapData 定义URL映射数据操作的接口规范
type IUrlMapData interface {
	// GenerateID 生成新的URL映射记录并返回自增ID
	GenerateID(userID, now int64) (int64, error)

	// Update 更新指定URL映射记录
	Update(e UrlMapEntity) error

	// GetByID 通过ID查询URL映射记录
	GetByID(id int64) (UrlMapEntity, error)

	// GetByOriginal 通过原始URL查询映射记录
	GetByOriginal(originalUrl string) (UrlMapEntity, error)

	// IncrementTimes 增加指定记录的访问次数
	IncrementTimes(id int64, incrementTimes int, now int64) error
}

type urlMapData struct {
	log       log.ILogger // 日志记录器
	db        *sql.DB     // 数据库连接
	tableName string      // 表名
}

// newUrlMapData 创建URL映射数据操作对象
func newUrlMapData(log log.ILogger, db *sql.DB, tableName string) IUrlMapData {
	return &urlMapData{
		log:       log,
		db:        db,
		tableName: tableName,
	}
}

// GenerateID 创建新的URL映射记录
// 参数：
//   - userID: 用户ID（0表示公共链接）
//   - now: 当前时间戳
//
// 返回：
//   - 新生成的记录ID
//   - 错误信息（数据库操作失败时）
func (d *urlMapData) GenerateID(userID, now int64) (int64, error) {
	if userID != 0 {
		sqlStr := fmt.Sprintf("insert into %s (user_id,create_at,update_at)values(?,?,?)", d.tableName)
		res, err := d.db.Exec(sqlStr, userID, now, now)
		if err != nil {
			d.log.Error(zerror.NewByErr(err))
			return 0, err
		}
		return res.LastInsertId()
	} else {
		sqlStr := fmt.Sprintf("insert into %s (create_at,update_at)values(?,?)", d.tableName)
		res, err := d.db.Exec(sqlStr, now, now)
		if err != nil {
			d.log.Error(zerror.NewByErr(err))
			return 0, err
		}
		return res.LastInsertId()
	}

}

// Update 更新URL映射记录
// 参数：
//   - e: 需要更新的实体对象
//
// 返回：
//   - 错误信息（数据库操作失败时）
func (d *urlMapData) Update(e UrlMapEntity) error {
	sqlStr := fmt.Sprintf("update %s set short_key=?,original_url=?,update_at=? where id=?", d.tableName)
	_, err := d.db.Exec(sqlStr, e.ShortKey, e.OriginalUrl, e.UpdateAt, e.ID)
	if err != nil {
		d.log.Error(zerror.NewByErr(err))
		return err
	}
	return nil
}

// GetByID 通过ID查询URL映射记录
// 参数：
//   - id: 记录ID
//
// 返回：
//   - 查询到的实体对象（未找到时各字段为零值）
//   - 错误信息（数据库操作失败时）
func (d *urlMapData) GetByID(id int64) (UrlMapEntity, error) {
	sqlStr := fmt.Sprintf("select original_url from %s where id = ?", d.tableName)
	row := d.db.QueryRow(sqlStr, id)
	entity := UrlMapEntity{}
	var originalUrl sql.NullString
	err := row.Scan(&originalUrl)
	// 特殊处理未找到的情况
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		d.log.Error(zerror.NewByErr(err))
		return entity, err
	}
	if originalUrl.Valid {
		entity.OriginalUrl = originalUrl.String
	}
	return entity, nil
}

// GetByOriginal 通过原始URL查询映射记录
// 参数：
//   - originalUrl: 原始URL
//
// 返回：
//   - 查询到的实体对象（未找到时各字段为零值）
//   - 错误信息（数据库操作失败时）
func (d *urlMapData) GetByOriginal(originalUrl string) (UrlMapEntity, error) {
	sqlStr := fmt.Sprintf("select id, short_key from %s where original_url = ?", d.tableName)
	row := d.db.QueryRow(sqlStr, originalUrl)
	entity := UrlMapEntity{}
	var shortKey sql.NullString
	err := row.Scan(&entity.ID, &shortKey)
	// 处理非预期错误
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		d.log.Error(zerror.NewByErr(err))
		return entity, err
	}
	if shortKey.Valid {
		entity.ShortKey = shortKey.String
	}
	return entity, nil
}

// IncrementTimes 增加指定记录的访问次数
// 参数：
//   - id: 记录ID
//   - incrementTimes: 需要增加的次数
//   - now: 当前时间戳
//
// 返回：
//   - 错误信息（数据库操作失败时）
func (d *urlMapData) IncrementTimes(id int64, incrementTimes int, now int64) error {
	sqlStr := fmt.Sprintf("update %s set times = times + ?, update_at=? where id = ?", d.tableName)
	_, err := d.db.Exec(sqlStr, incrementTimes, now, id)
	if err != nil {
		d.log.Error(zerror.NewByErr(err))
		return err
	}
	return nil
}
