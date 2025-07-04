package domain

import (
	"github.com/toheart/functrace/domain/model"
)

// TraceRepository 跟踪数据仓储接口
type TraceRepository interface {
	// SaveTrace 保存跟踪数据
	SaveTrace(trace *model.TraceData) (int64, error)

	// UpdateTraceTimeCost 更新跟踪时间成本
	UpdateTraceTimeCost(id int64, timeCost string) error

	// FindRootFunctionsByGID 根据GID查找根函数
	FindRootFunctionsByGID(gid uint64) ([]model.TraceData, error)
}

// ParamRepository 参数数据仓储接口
type ParamRepository interface {
	// SaveParam 保存参数数据
	SaveParam(param *model.ParamStoreData) (int64, error)

	// FindParamsByTraceID 根据跟踪ID查找参数
	FindParamsByTraceID(traceId int64) ([]model.ParamStoreData, error)

	// SaveParamCache 保存参数缓存
	SaveParamCache(cache *model.ParamCache) (int64, error)

	// FindParamCacheByAddr 根据地址查找参数缓存
	FindParamCacheByAddr(addr string) (*model.ParamCache, error)

	// DeleteParamCacheByTraceID 根据跟踪ID删除参数缓存
	DeleteParamCacheByTraceID(traceId int64) error

	// UpdateParamCache 更新参数缓存
	UpdateParamCache(cache *model.ParamCache) error
}

// GoroutineRepository 协程数据仓储接口
type GoroutineRepository interface {
	// SaveGoroutine 保存协程数据
	SaveGoroutine(goroutine *model.GoroutineTrace) (int64, error)

	// UpdateGoroutineTimeCost 更新协程时间成本
	UpdateGoroutineTimeCost(id int64, timeCost string, isFinished int) error

	// FindGoroutineByID 根据ID查找协程
	FindGoroutineByID(id int64) (*model.GoroutineTrace, error)
}

// RepositoryFactory 仓储工厂接口
type RepositoryFactory interface {
	// 初始化数据库
	Initialize() error

	// GetTraceRepository 获取跟踪数据仓储
	GetTraceRepository() TraceRepository

	// GetParamRepository 获取参数数据仓储
	GetParamRepository() ParamRepository

	// GetGoroutineRepository 获取协程数据仓储
	GetGoroutineRepository() GoroutineRepository

	// 关闭数据库连接
	Close() error
}
