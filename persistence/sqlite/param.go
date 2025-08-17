package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/domain/model"
)

// ParamRepository 是SQLite实现的参数数据仓储
type ParamRepository struct {
	db *sql.DB
}

// NewParamRepository 创建一个新的SQLite参数数据仓储
func NewParamRepository(db *sql.DB) domain.ParamRepository {
	return &ParamRepository{
		db: db,
	}
}

// SaveParam 保存参数数据
func (r *ParamRepository) SaveParam(param *model.ParamStoreData) (int64, error) {
	result, err := r.db.Exec(
		SQLInsertParam,
		param.ID,
		param.TraceID,
		param.Position,
		param.Data,
		param.IsReceiver,
		param.BaseID,
	)
	if err != nil {
		return 0, fmt.Errorf("save param error: %w", err)
	}

	return result.LastInsertId()
}

// FindParamsByTraceID 根据跟踪ID查找参数
func (r *ParamRepository) FindParamsByTraceID(traceId int64) ([]model.ParamStoreData, error) {
	rows, err := r.db.Query("SELECT id, traceId, position, data, isReceiver, baseId FROM ParamStore WHERE traceId = ?", traceId)
	if err != nil {
		return nil, fmt.Errorf("find params by trace id error: %w", err)
	}
	defer rows.Close()

	var result []model.ParamStoreData
	for rows.Next() {
		var param model.ParamStoreData

		if err := rows.Scan(&param.ID, &param.TraceID, &param.Position, &param.Data, &param.IsReceiver, &param.BaseID); err != nil {
			return nil, fmt.Errorf("scan param data error: %w", err)
		}
		result = append(result, param)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate param data result error: %w", err)
	}

	return result, nil
}

// SaveParamCache 保存参数缓存
func (r *ParamRepository) SaveParamCache(cache *model.ParamCache) (int64, error) {
	result, err := r.db.Exec(
		SQLInsertParamCache,
		cache.Addr,
		cache.BaseID,
		cache.Data,
	)
	if err != nil {
		return 0, fmt.Errorf("save param cache error: %w", err)
	}

	return result.LastInsertId()
}

// FindParamCacheByAddr 根据地址查找参数缓存
func (r *ParamRepository) FindParamCacheByAddr(addr string) (*model.ParamCache, error) {
	row := r.db.QueryRow(SQLSelectParamCacheByAddr, addr)

	var cache model.ParamCache
	err := row.Scan(&cache.ID, &cache.Addr, &cache.BaseID, &cache.Data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 没有找到缓存，返回 nil
		}
		return nil, fmt.Errorf("find param cache by addr error: %w", err)
	}

	return &cache, nil
}

// DeleteParamCacheByAddr 根据地址删除参数缓存
func (r *ParamRepository) DeleteParamCacheByAddr(addr string) error {
	_, err := r.db.Exec(SQLDeleteParamCacheByAddr, addr)
	if err != nil {
		return fmt.Errorf("delete param cache by addr error: %w", err)
	}
	return nil
}
