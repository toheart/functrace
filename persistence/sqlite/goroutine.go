package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/domain/model"
)

// GoroutineRepository 是SQLite实现的协程数据仓储
type GoroutineRepository struct {
	db *sql.DB
}

// NewGoroutineRepository 创建一个新的SQLite协程数据仓储
func NewGoroutineRepository(db *sql.DB) domain.GoroutineRepository {
	return &GoroutineRepository{
		db: db,
	}
}

// SaveGoroutine 保存协程数据
func (r *GoroutineRepository) SaveGoroutine(goroutine *model.GoroutineTrace) (int64, error) {
	result, err := r.db.Exec(
		SQLInsertGoroutine,
		goroutine.ID,
		goroutine.OriginGID,
		goroutine.CreateTime,
		goroutine.IsFinished,
		goroutine.InitFuncName,
	)
	if err != nil {
		return 0, fmt.Errorf("save goroutine error: %w", err)
	}

	return result.LastInsertId()
}

// UpdateGoroutineTimeCost 更新协程时间成本
func (r *GoroutineRepository) UpdateGoroutineTimeCost(id int64, timeCost string, isFinished int) error {
	_, err := r.db.Exec(SQLUpdateGoroutineTimeCost, timeCost, isFinished, id)
	if err != nil {
		return fmt.Errorf("update goroutine time cost error: %w", err)
	}
	return nil
}

// FindGoroutineByID 根据ID查找协程
func (r *GoroutineRepository) FindGoroutineByID(id int64) (*model.GoroutineTrace, error) {
	rows, err := r.db.Query("SELECT id, originGid, createTime, isFinished, initFuncName FROM GoroutineTrace WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("find goroutine by id error: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("goroutine data not found: id=%d", id)
	}

	var goroutine model.GoroutineTrace

	if err := rows.Scan(&goroutine.ID, &goroutine.OriginGID, &goroutine.CreateTime, &goroutine.IsFinished, &goroutine.InitFuncName); err != nil {
		return nil, fmt.Errorf("scan goroutine data error: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("process goroutine data error: %w", err)
	}

	return &goroutine, nil
}
