package memory

import (
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
)

// MemTraceRepository 实现跟踪仓储的Mock
type MemTraceRepository struct {
	logger *logrus.Logger
}

// NewMockTraceRepository 创建新的Mock跟踪仓储
func NewMockTraceRepository(logger *logrus.Logger) *MemTraceRepository {
	return &MemTraceRepository{
		logger: logger,
	}
}

// SaveTrace 保存跟踪数据
func (r *MemTraceRepository) SaveTrace(trace *model.TraceData) (int64, error) {
	r.logger.WithField("trace", trace).Info("Mock保存跟踪数据")
	return 1, nil
}

// UpdateTraceTimeCost 更新跟踪时间成本
func (r *MemTraceRepository) UpdateTraceTimeCost(id int64, timeCost string) error {
	r.logger.WithFields(logrus.Fields{
		"id":       id,
		"timeCost": timeCost,
	}).Info("Mock更新跟踪时间成本")
	return nil
}

// FindRootFunctionsByGID 查找指定GID的根函数
func (r *MemTraceRepository) FindRootFunctionsByGID(gid uint64) ([]model.TraceData, error) {
	r.logger.WithField("gid", gid).Info("Mock查找根函数")
	return []model.TraceData{
		*model.NewTraceData(1, "MockRootFunction", gid, 0, 1, 0, "2023-01-01T00:00:00Z", "seq001"),
	}, nil
}

// MemParamRepository 实现参数仓储的Mock
type MemParamRepository struct {
	logger *logrus.Logger
}

// FindParamsByTraceID implements domain.ParamRepository.
func (r *MemParamRepository) FindParamsByTraceID(traceId int64) ([]model.ParamStoreData, error) {
	r.logger.WithField("traceId", traceId).Info("Mock查找参数")
	return []model.ParamStoreData{
		*model.NewParamStoreData(1, 0, "MockParam", false, 0),
	}, nil
}

// NewMockParamRepository 创建新的Mock参数仓储
func NewMockParamRepository(logger *logrus.Logger) *MemParamRepository {
	return &MemParamRepository{
		logger: logger,
	}
}

// SaveParam 保存参数数据
func (r *MemParamRepository) SaveParam(param *model.ParamStoreData) (int64, error) {
	r.logger.WithField("param", param).Info("Mock保存参数数据")
	return 1, nil
}

// SaveParamCache 保存参数缓存
func (r *MemParamRepository) SaveParamCache(cache *model.ParamCache) (int64, error) {
	r.logger.WithField("cache", cache).Info("Mock保存参数缓存")
	return 1, nil
}

// FindParamCacheByAddr 根据地址查找参数缓存
func (r *MemParamRepository) FindParamCacheByAddr(addr string) (*model.ParamCache, error) {
	r.logger.WithField("addr", addr).Info("Mock查找参数缓存")
	return &model.ParamCache{
		ID:      1,
		Addr:    addr,
		TraceID: 1,
		BaseID:  1,
		Data:    "MockCacheData",
	}, nil
}

// DeleteParamCacheByTraceID 根据跟踪ID删除参数缓存
func (r *MemParamRepository) DeleteParamCacheByTraceID(traceId int64) error {
	r.logger.WithField("traceId", traceId).Info("Mock删除参数缓存")
	return nil
}

// UpdateParamCache 更新参数缓存
func (r *MemParamRepository) UpdateParamCache(cache *model.ParamCache) error {
	r.logger.WithField("cache", cache).Info("Mock更新参数缓存")
	return nil
}

// MemGoroutineRepository 实现协程仓储的Mock
type MemGoroutineRepository struct {
	logger *logrus.Logger
}

// FindGoroutineByID implements domain.GoroutineRepository.
func (r *MemGoroutineRepository) FindGoroutineByID(id int64) (*model.GoroutineTrace, error) {
	r.logger.WithField("id", id).Info("Mock查找协程")
	return &model.GoroutineTrace{
		ID:           id,
		OriginGID:    1,
		TimeCost:     "100ms",
		CreateTime:   "2023-01-01T00:00:00Z",
		IsFinished:   0,
		InitFuncName: "MockInitFunction",
	}, nil
}

// NewMockGoroutineRepository 创建新的Mock协程仓储
func NewMockGoroutineRepository(logger *logrus.Logger) *MemGoroutineRepository {
	return &MemGoroutineRepository{
		logger: logger,
	}
}

// SaveGoroutine 保存协程数据
func (r *MemGoroutineRepository) SaveGoroutine(goroutine *model.GoroutineTrace) (int64, error) {
	r.logger.WithField("goroutine", goroutine).Info("Mock保存协程数据")
	return 1, nil
}

// UpdateGoroutineTimeCost 更新协程时间成本
func (r *MemGoroutineRepository) UpdateGoroutineTimeCost(id int64, timeCost string, isFinished int) error {
	r.logger.WithFields(logrus.Fields{
		"id":         id,
		"timeCost":   timeCost,
		"isFinished": isFinished,
	}).Info("Mock更新协程时间成本")
	return nil
}
