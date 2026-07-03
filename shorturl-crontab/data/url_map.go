package data

import (
	"database/sql"
	"fmt"
	"github.com/pkg/errors"
)

type data struct {
	db *sql.DB
}

func NewData(db *sql.DB) *data {
	return &data{db: db}
}

func (d *data) GetMaxID(tableName string) (int64, error) {
	sqlStr := fmt.Sprintf("select max(id) as id from %s", tableName)
	var id sql.NullInt64
	row := d.db.QueryRow(sqlStr)
	err := row.Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if id.Valid {
		return id.Int64, nil
	}
	return 0, nil
}

// IncrementTimes 把 Redis 聚合出的访问次数增量写回指定短链表。
//
// tableName 只能来自 crontab 内部维护的 url_map / url_map_user 表名列表，不能接收外部请求参数。
// incrementTimes 是本轮 flush 扫描到的快照值；写库成功后调用方才会从 Redis 扣减同样的快照值，
// 因此失败重试会保留 Redis 计数，避免访问统计丢失。
func (d *data) IncrementTimes(tableName string, id int64, incrementTimes int64, now int64) error {
	sqlStr := fmt.Sprintf("update %s set times = times + ?, update_at=? where id = ?", tableName)
	_, err := d.db.Exec(sqlStr, incrementTimes, now, id)
	return err
}
