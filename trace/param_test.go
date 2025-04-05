package trace

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/domain/model"
	"github.com/toheart/functrace/spew"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// 测试设置
func setupTestTraceInstance() *TraceInstance {
	// 创建一个测试用的 TraceInstance
	t := &TraceInstance{
		indentations:     make(map[uint64]*TraceIndent),
		log:              logrus.New(),
		GoroutineRunning: make(map[uint64]*GoroutineInfo),
		paramCache:       make(map[string]*receiverInfo),
		IgnoreNames:      []string{},
		spewConfig:       &spew.ConfigState{},
	}
	// 设置 spewConfig
	t.spewConfig.DisableMethods = true
	t.spewConfig.DisableCapacities = true
	t.spewConfig.EnableJSONOutput = true
	t.spewConfig.MaxDepth = 10
	return t
}

// MockParamRepository 是 ParamRepository 的模拟实现
type MockParamRepository struct {
	mock.Mock
}

func (m *MockParamRepository) SaveParam(param *model.ParamStoreData) (int64, error) {
	args := m.Called(param)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockParamRepository) FindParamsByTraceID(traceId int64) ([]model.ParamStoreData, error) {
	args := m.Called(traceId)
	return args.Get(0).([]model.ParamStoreData), args.Error(1)
}

// MockTraceRepository 是 TraceRepository 的模拟实现
type MockTraceRepository struct {
	mock.Mock
}

func (m *MockTraceRepository) SaveTrace(trace *model.TraceData) (int64, error) {
	args := m.Called(trace)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTraceRepository) UpdateTraceTimeCost(id int64, timeCost string) error {
	args := m.Called(id, timeCost)
	return args.Error(0)
}

func (m *MockTraceRepository) FindRootFunctionsByGID(gid uint64) ([]model.TraceData, error) {
	args := m.Called(gid)
	return args.Get(0).([]model.TraceData), args.Error(1)
}

// MockGoroutineRepository 是 GoroutineRepository 的模拟实现
type MockGoroutineRepository struct {
	mock.Mock
}

func (m *MockGoroutineRepository) SaveGoroutine(goroutine *model.GoroutineTrace) (int64, error) {
	args := m.Called(goroutine)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockGoroutineRepository) UpdateGoroutineTimeCost(id int64, timeCost string, isFinished int) error {
	args := m.Called(id, timeCost, isFinished)
	return args.Error(0)
}

func (m *MockGoroutineRepository) FindGoroutineByID(id int64) (*model.GoroutineTrace, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.GoroutineTrace), args.Error(1)
}

// 模拟 RepositoryFactory
type mockRepositoryFactory struct {
	mock.Mock
	paramRepo     *MockParamRepository
	traceRepo     *MockTraceRepository
	goroutineRepo *MockGoroutineRepository
}

func (m *mockRepositoryFactory) Initialize() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockRepositoryFactory) GetParamRepository() domain.ParamRepository {
	return m.paramRepo
}

func (m *mockRepositoryFactory) GetTraceRepository() domain.TraceRepository {
	if m.traceRepo != nil {
		return m.traceRepo
	}
	args := m.Called()
	return args.Get(0).(domain.TraceRepository)
}

func (m *mockRepositoryFactory) GetGoroutineRepository() domain.GoroutineRepository {
	if m.goroutineRepo != nil {
		return m.goroutineRepo
	}
	args := m.Called()
	return args.Get(0).(domain.GoroutineRepository)
}

func (m *mockRepositoryFactory) Close() error {
	args := m.Called()
	return args.Error(0)
}

// 测试 DealNormalMethod 函数
func TestDealNormalMethod(t *testing.T) {
	// 设置测试环境
	traceInstance := setupTestTraceInstance()

	// 创建模拟的仓储工厂和参数仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &mockRepositoryFactory{paramRepo: mockParamRepo}
	mockFactory.On("Close").Return(nil)

	// 替换全局变量
	oldFactory := repositoryFactory
	repositoryFactory = mockFactory
	defer func() { repositoryFactory = oldFactory }()

	// 设置预期行为
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1), nil)

	// 执行被测试的函数
	traceID := int64(123)
	params := []interface{}{"test", 42, true}
	traceInstance.DealNormalMethod(traceID, params)

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

	// 验证预期被满足
	mockParamRepo.AssertNumberOfCalls(t, "SaveParam", len(params))
	mockParamRepo.AssertExpectations(t)
}

// 测试 DealValueMethod 函数
func TestDealValueMethod(t *testing.T) {
	// 设置测试环境
	traceInstance := setupTestTraceInstance()

	// 创建模拟的仓储工厂和参数仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &mockRepositoryFactory{paramRepo: mockParamRepo}
	mockFactory.On("Close").Return(nil)

	// 替换全局变量
	oldFactory := repositoryFactory
	repositoryFactory = mockFactory
	defer func() { repositoryFactory = oldFactory }()

	// 设置预期行为
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1), nil)

	// 执行被测试的函数
	traceID := int64(123)
	params := []interface{}{"test", 42, true}
	traceInstance.DealValueMethod(traceID, params)

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

	// 验证预期被满足
	mockParamRepo.AssertNumberOfCalls(t, "SaveParam", len(params))
	mockParamRepo.AssertExpectations(t)
}

// 测试 DealPointerMethod 函数 - 空参数列表
func TestDealPointerMethodEmptyParams(t *testing.T) {
	// 设置测试环境
	traceInstance := setupTestTraceInstance()

	// 创建模拟的仓储工厂和参数仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &mockRepositoryFactory{paramRepo: mockParamRepo}
	mockFactory.On("Close").Return(nil)

	// 替换全局变量
	oldFactory := repositoryFactory
	repositoryFactory = mockFactory
	defer func() { repositoryFactory = oldFactory }()

	// 执行被测试的函数 - 空参数列表
	traceID := int64(123)
	params := []interface{}{}
	traceInstance.DealPointerMethod(traceID, params)

	// 验证没有调用 SaveParam
	mockParamRepo.AssertNotCalled(t, "SaveParam")
}

// 测试 DealPointerMethod 函数 - 有参数
func TestDealPointerMethodWithParams(t *testing.T) {
	// 设置测试环境
	traceInstance := setupTestTraceInstance()

	// 创建模拟的仓储工厂和参数仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &mockRepositoryFactory{paramRepo: mockParamRepo}
	mockFactory.On("Close").Return(nil)

	// 替换全局变量
	oldFactory := repositoryFactory
	repositoryFactory = mockFactory
	defer func() { repositoryFactory = oldFactory }()

	// 设置预期行为
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1), nil)

	// 创建一个接收者对象和其他参数
	receiver := struct{ Value string }{"receiver"}
	otherParam := "other"
	params := []interface{}{receiver, otherParam}

	// 执行被测试的函数
	traceID := int64(123)
	traceInstance.DealPointerMethod(traceID, params)

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

	// 验证预期被满足 - 应该调用2次 (接收者 + 其他参数)
	mockParamRepo.AssertNumberOfCalls(t, "SaveParam", len(params))
	mockParamRepo.AssertExpectations(t)
}

// 测试 DealPointerMethod 函数 - 缓存场景
func TestDealPointerMethodWithCache(t *testing.T) {
	// 设置测试环境
	traceInstance := setupTestTraceInstance()

	// 创建模拟的仓储工厂和参数仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &mockRepositoryFactory{paramRepo: mockParamRepo}
	mockFactory.On("Close").Return(nil)

	// 替换全局变量
	oldFactory := repositoryFactory
	repositoryFactory = mockFactory
	defer func() { repositoryFactory = oldFactory }()

	// 设置预期行为
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1), nil)

	// 创建一个接收者对象
	receiver := &struct{ Value string }{"receiver"}
	addr := fmt.Sprintf("%d", reflect.ValueOf(receiver).Pointer())

	// 首次调用 - 应该将接收者添加到缓存
	traceID1 := int64(123)
	traceInstance.DealPointerMethod(traceID1, []interface{}{receiver})
	t.Logf("map: %v", traceInstance.paramCache)
	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

	// 验证缓存中存在接收者
	cacheItem, exists := traceInstance.paramCache[addr]
	assert.True(t, exists, "接收者应该被添加到缓存中")
	assert.Equal(t, traceID1, cacheItem.BaseID, "缓存中的BaseID应该等于第一次跟踪ID")

	// 第二次调用 - 使用相同的接收者，但值发生变化
	receiver.Value = "modified"
	traceID2 := int64(456)
	traceInstance.DealPointerMethod(traceID2, []interface{}{receiver})

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

	// 验证 SaveParam 调用了2次
	mockParamRepo.AssertNumberOfCalls(t, "SaveParam", 2)
	mockParamRepo.AssertExpectations(t)
}

// 测试 createJSONPatch 函数
func TestCreateJSONPatch(t *testing.T) {
	// 测试用例1: 相同的字符串
	original := `{"value":"test"}`
	modified := `{"value":"test"}`
	patch, err := createJSONPatch(original, modified)
	t.Logf("patch: %s", patch)
	assert.NoError(t, err, "相同字符串不应该产生错误")
	assert.Empty(t, patch, "相同字符串应该返回空补丁")

	// 测试用例2: 修改后的字符串 - 简单修改
	original = `{"value":"test"}`
	modified = `{"value":"modified"}`
	patch, err = createJSONPatch(original, modified)
	t.Logf("patch: %s", patch)
	assert.NoError(t, err, "修改后的字符串不应该产生错误")
	assert.NotEmpty(t, patch, "修改后的字符串应该返回非空补丁")
	assert.Contains(t, patch, "modified", "补丁应该包含修改后的值")

	// 测试用例3: 添加新字段
	original = `{"value":"test"}`
	modified = `{"value":"test","newField":"added"}`
	patch, err = createJSONPatch(original, modified)
	t.Logf("patch: %s", patch)
	assert.NoError(t, err, "添加新字段不应该产生错误")
	assert.NotEmpty(t, patch, "添加新字段应该返回非空补丁")
	assert.Contains(t, patch, "newField", "补丁应该包含新字段名")
	assert.Contains(t, patch, "added", "补丁应该包含新字段值")

	// 测试用例4: 删除字段
	original = `{"value":"test","toRemove":"remove"}`
	modified = `{"value":"test"}`
	patch, err = createJSONPatch(original, modified)
	t.Logf("patch: %s", patch)
	assert.NoError(t, err, "删除字段不应该产生错误")
	assert.NotEmpty(t, patch, "删除字段应该返回非空补丁")
	assert.Contains(t, patch, "null", "补丁应该表示删除")

	// 测试用例5: 复杂对象修改
	original = `{"obj":{"nested":"value","arr":[1,2,3]}}`
	modified = `{"obj":{"nested":"changed","arr":[1,2,3,4]}}`
	patch, err = createJSONPatch(original, modified)
	t.Logf("patch: %s", patch)
	assert.NoError(t, err, "复杂对象修改不应该产生错误")
	assert.NotEmpty(t, patch, "复杂对象修改应该返回非空补丁")
	assert.Contains(t, patch, "changed", "补丁应该包含修改后的嵌套值")

	// 测试用例6: 格式错误的JSON
	original = `this is not json`
	modified = `{"value":"test"}`
	patch, err = createJSONPatch(original, modified)
	t.Logf("patch: %s", patch)
	assert.Error(t, err, "格式错误的JSON应该产生错误")
	assert.Empty(t, patch, "格式错误的JSON应该返回空补丁")
}

// 测试 storeParams 函数 - 正常情况
func TestStoreParams(t *testing.T) {
	// 设置测试环境
	traceInstance := setupTestTraceInstance()

	// 创建模拟的仓储工厂和参数仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &mockRepositoryFactory{paramRepo: mockParamRepo}
	mockFactory.On("Close").Return(nil)

	// 替换全局变量
	oldFactory := repositoryFactory
	repositoryFactory = mockFactory
	defer func() { repositoryFactory = oldFactory }()

	// 设置预期行为 - 成功保存
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1), nil)

	// 创建测试参数列表
	paramList := []model.ParamStoreData{
		{TraceID: 1, Position: 0, Data: "data1", IsReceiver: true, BaseID: 0},
		{TraceID: 1, Position: 1, Data: "data2", IsReceiver: false, BaseID: 0},
		{TraceID: 1, Position: 2, Data: "data3", IsReceiver: false, BaseID: 0},
	}

	for _, param := range paramList {
		// 执行被测试的函数
		traceInstance.storeParam(&param)
	}

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

	// 验证预期被满足
	mockParamRepo.AssertNumberOfCalls(t, "SaveParam", len(paramList))
	mockParamRepo.AssertExpectations(t)

	// 检查每次调用的参数
	for i, call := range mockParamRepo.Calls {
		if i < len(paramList) {
			param := call.Arguments.Get(0).(*model.ParamStoreData)
			assert.Equal(t, paramList[i].TraceID, param.TraceID, "TraceID 应该匹配")
			assert.Equal(t, paramList[i].Position, param.Position, "Position 应该匹配")
			assert.Equal(t, paramList[i].Data, param.Data, "Data 应该匹配")
			assert.Equal(t, paramList[i].IsReceiver, param.IsReceiver, "IsReceiver 应该匹配")
			assert.Equal(t, paramList[i].BaseID, param.BaseID, "BaseID 应该匹配")
		}
	}
}

// 测试 storeParams 函数 - 保存失败
func TestStoreParamsWithError(t *testing.T) {
	// 设置测试环境
	traceInstance := setupTestTraceInstance()

	// 创建一个测试日志记录器以捕获日志输出
	testLogger := logrus.New()
	var logBuffer bytes.Buffer
	testLogger.Out = &logBuffer
	testLogger.Level = logrus.ErrorLevel
	traceInstance.log = testLogger

	// 创建模拟的仓储工厂和参数仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &mockRepositoryFactory{paramRepo: mockParamRepo}
	mockFactory.On("Close").Return(nil)

	// 替换全局变量
	oldFactory := repositoryFactory
	repositoryFactory = mockFactory
	defer func() { repositoryFactory = oldFactory }()

	// 设置预期行为 - 返回错误
	expectedError := fmt.Errorf("保存失败")
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(0), expectedError)

	// 创建测试参数列表
	paramList := []model.ParamStoreData{
		{TraceID: 1, Position: 0, Data: "data1", IsReceiver: false, BaseID: 0},
	}

	// 执行被测试的函数
	for _, param := range paramList {
		traceInstance.storeParam(&param)
	}

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

	// 验证预期被满足
	mockParamRepo.AssertNumberOfCalls(t, "SaveParam", len(paramList))
	mockParamRepo.AssertExpectations(t)

	// 验证日志中包含错误信息
	assert.Contains(t, logBuffer.String(), "保存参数数据失败", "日志应该包含错误信息")
	assert.Contains(t, logBuffer.String(), expectedError.Error(), "日志应该包含具体错误信息")
}
