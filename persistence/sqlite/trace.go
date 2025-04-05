package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/domain/model"
)

// TraceRepository 是SQLite实现的跟踪数据仓储
type TraceRepository struct {
	db *sql.DB
}

// NewTraceRepository 创建一个新的SQLite跟踪数据仓储
func NewTraceRepository(db *sql.DB) domain.TraceRepository {
	return &TraceRepository{
		db: db,
	}
}

// SaveTrace 保存跟踪数据
func (r *TraceRepository) SaveTrace(trace *model.TraceData) (int64, error) {
	result, err := r.db.Exec(
		SQLInsertTrace,
		trace.ID,
		trace.Name,
		trace.GID,
		trace.Indent,
		trace.ParamsCount,
		trace.ParentId,
		trace.CreatedAt,
		trace.Seq,
	)
	if err != nil {
		return 0, fmt.Errorf("save trace error: %w", err)
	}

	return result.LastInsertId()
}

// UpdateTraceTimeCost 更新跟踪时间成本
func (r *TraceRepository) UpdateTraceTimeCost(id int64, timeCost string) error {
	_, err := r.db.Exec(SQLUpdateTimeCost, timeCost, id)
	if err != nil {
		return fmt.Errorf("update trace time cost error: %w", err)
	}
	return nil
}

// FindRootFunctionsByGID 根据GID查找根函数
func (r *TraceRepository) FindRootFunctionsByGID(gid uint64) ([]model.TraceData, error) {
	rows, err := r.db.Query(SQLQueryRootFunctions, gid)
	if err != nil {
		return nil, fmt.Errorf("find root functions by gid error: %w", err)
	}
	defer rows.Close()

	var result []model.TraceData
	for rows.Next() {
		var trace model.TraceData
		trace.GID = gid
		trace.Indent = 0

		if err := rows.Scan(&trace.ID, &trace.TimeCost); err != nil {
			return nil, fmt.Errorf("scan root functions data error: %w", err)
		}
		result = append(result, trace)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate root functions result error: %w", err)
	}

	return result, nil
}
