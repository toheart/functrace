package memory

import (
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/domain/model"
)

// 确保MockDatabase实现了IDatabase接口
var _ domain.RepositoryFactory = (*MemDatabase)(nil)

// MemDatabase 模拟数据库实现
type MemDatabase struct {
	logger              *logrus.Logger
	traceData           map[int64]*model.TraceData
	params              map[int64]*model.ParamStoreData
	goroutines          map[int64]*model.GoroutineTrace
	traceRepository     *MemTraceRepository
	paramRepository     *MemParamRepository
	goroutineRepository *MemGoroutineRepository
	nextID              int64
}

// NewMockDatabase 创建新的模拟数据库
func NewMockDatabase(logger *logrus.Logger) domain.RepositoryFactory {
	return &MemDatabase{
		logger:              logger,
		traceData:           make(map[int64]*model.TraceData),
		params:              make(map[int64]*model.ParamStoreData),
		goroutines:          make(map[int64]*model.GoroutineTrace),
		traceRepository:     NewMockTraceRepository(logger),
		paramRepository:     NewMockParamRepository(logger),
		goroutineRepository: NewMockGoroutineRepository(logger),
		nextID:              1,
	}
}

// Initialize 初始化数据库
func (m *MemDatabase) Initialize() error {
	return nil // 模拟实现无需初始化
}

// Close 关闭数据库连接
func (m *MemDatabase) Close() error {
	return nil // 模拟实现无需关闭
}

// getNextID 获取下一个ID
func (m *MemDatabase) getNextID() int64 {
	id := m.nextID
	m.nextID++
	return id
}

func (m *MemDatabase) GetGoroutineRepository() domain.GoroutineRepository {
	return m.goroutineRepository
}

func (m *MemDatabase) GetTraceRepository() domain.TraceRepository {
	return m.traceRepository
}

func (m *MemDatabase) GetParamRepository() domain.ParamRepository {
	return m.paramRepository
}
