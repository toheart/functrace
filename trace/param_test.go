package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/domain/model"
)

// MockParamRepository 模拟参数仓储
type MockParamRepository struct {
	mock.Mock
}

func (m *MockParamRepository) SaveParam(param *model.ParamStoreData) (int64, error) {
	args := m.Called(param)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockParamRepository) FindParamCacheByAddr(addr string) (*model.ParamCache, error) {
	args := m.Called(addr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ParamCache), args.Error(1)
}

func (m *MockParamRepository) SaveParamCache(cache *model.ParamCache) (int64, error) {
	args := m.Called(cache)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockParamRepository) DeleteParamCacheByAddr(addr string) error {
	args := m.Called(addr)
	return args.Error(0)
}

func (m *MockParamRepository) FindParamsByTraceID(traceId int64) ([]model.ParamStoreData, error) {
	args := m.Called(traceId)
	return args.Get(0).([]model.ParamStoreData), args.Error(1)
}

// MockRepositoryFactory 模拟仓储工厂
type MockRepositoryFactory struct {
	mock.Mock
	paramRepo *MockParamRepository
}

func (m *MockRepositoryFactory) GetParamRepository() domain.ParamRepository {
	return m.paramRepo
}

func (m *MockRepositoryFactory) GetTraceRepository() domain.TraceRepository {
	return nil
}

func (m *MockRepositoryFactory) GetGoroutineRepository() domain.GoroutineRepository {
	return nil
}

func (m *MockRepositoryFactory) Initialize() error {
	return nil
}

func (m *MockRepositoryFactory) Close() error {
	return nil
}

// TestUser 测试用的结构体
type TestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (u *TestUser) GetInfo() string {
	return u.Name
}

// setupTestTraceInstance 设置测试用的TraceInstance
func setupTestTraceInstance() (*TraceInstance, *MockParamRepository) {
	// 创建mock仓储
	mockParamRepo := &MockParamRepository{}
	mockFactory := &MockRepositoryFactory{
		paramRepo: mockParamRepo,
	}

	// 设置全局仓储工厂
	repositoryFactory = mockFactory

	// 创建测试实例
	instance := &TraceInstance{
		config: &Config{
			InsertMode: SyncMode, // 使用同步模式便于测试
		},
		log: initializeLogger(),
	}
	instance.gParamId.Store(1000) // 设置初始ID

	return instance, mockParamRepo
}

// TestDealPointerMethod_EmptyParams 测试空参数列表
func TestDealPointerMethod_EmptyParams(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	// 执行测试
	traceInstance.DealPointerMethod(123, []interface{}{})

	// 验证没有调用任何仓储方法
	mockParamRepo.AssertNotCalled(t, "SaveParam")
	mockParamRepo.AssertNotCalled(t, "FindParamCacheByAddr")
	mockParamRepo.AssertNotCalled(t, "SaveParamCache")
}

// TestDealPointerMethod_NonPointerReceiver 测试非指针接收者
func TestDealPointerMethod_NonPointerReceiver(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	// 使用值类型作为接收者
	user := TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user, "arg1", 42}

	// Mock DealNormalMethod的行为（通过验证SaveParam调用）
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1), nil).Times(3)

	// 执行测试
	traceInstance.DealPointerMethod(123, params)

	// 验证调用了SaveParam（说明回退到了DealNormalMethod）
	mockParamRepo.AssertExpectations(t)

	// 验证没有调用缓存相关方法
	mockParamRepo.AssertNotCalled(t, "FindParamCacheByAddr")
	mockParamRepo.AssertNotCalled(t, "SaveParamCache")
}

// TestDealPointerMethod_FirstTimePointer 测试首次遇到指针对象
func TestDealPointerMethod_FirstTimePointer(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	// 使用指针类型作为接收者
	user := &TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user, "arg1", 42}

	// 生成稳定键
	stableKey := traceInstance.getStableObjectKey(user)
	assert.NotEmpty(t, stableKey)
	assert.Contains(t, stableKey, "TestUser@")

	// Mock 缓存查询返回nil（首次遇到）
	mockParamRepo.On("FindParamCacheByAddr", stableKey).Return((*model.ParamCache)(nil), nil)

	// Mock 保存参数和缓存
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1001), nil).Times(3)
	mockParamRepo.On("SaveParamCache", mock.AnythingOfType("*model.ParamCache")).Return(int64(1), nil)

	// 执行测试
	traceInstance.DealPointerMethod(123, params)

	// 验证所有调用
	mockParamRepo.AssertExpectations(t)

	// 验证SaveParamCache被调用时的参数
	calls := mockParamRepo.Calls
	var cacheCall *mock.Call
	for _, call := range calls {
		if call.Method == "SaveParamCache" {
			cacheCall = &call
			break
		}
	}
	assert.NotNil(t, cacheCall)

	cache := cacheCall.Arguments[0].(*model.ParamCache)
	assert.Equal(t, stableKey, cache.Addr)
	assert.Equal(t, int64(1001), cache.BaseID)
	assert.NotEmpty(t, cache.Data)
}

// TestDealPointerMethod_CacheHit 测试缓存命中
func TestDealPointerMethod_CacheHit(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	// 使用指针类型作为接收者
	user := &TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user}

	stableKey := traceInstance.getStableObjectKey(user)

	// 创建原始缓存数据
	originalData := `{"id":1,"name":"original","age":20}`
	compressedOriginal := compress(originalData)

	existingCache := &model.ParamCache{
		Addr:   stableKey,
		BaseID: 500,
		Data:   compressedOriginal,
	}

	// Mock 缓存查询返回存在的缓存
	mockParamRepo.On("FindParamCacheByAddr", stableKey).Return(existingCache, nil)

	// Mock 保存参数
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1001), nil)

	// 执行测试
	traceInstance.DealPointerMethod(123, params)

	// 验证调用
	mockParamRepo.AssertExpectations(t)

	// 验证SaveParam被调用时的参数
	calls := mockParamRepo.Calls
	var saveCall *mock.Call
	for _, call := range calls {
		if call.Method == "SaveParam" {
			saveCall = &call
			break
		}
	}
	assert.NotNil(t, saveCall)

	paramData := saveCall.Arguments[0].(*model.ParamStoreData)
	assert.Equal(t, int64(123), paramData.TraceID)
	assert.Equal(t, 0, paramData.Position)
	assert.True(t, paramData.IsReceiver)
	assert.Equal(t, int64(500), paramData.BaseID) // 应该使用缓存的BaseID
	assert.NotEmpty(t, paramData.Data)

	// 验证没有调用SaveParamCache（因为缓存已存在）
	mockParamRepo.AssertNotCalled(t, "SaveParamCache")
}

// TestDealPointerMethod_CacheError 测试缓存查询错误
func TestDealPointerMethod_CacheError(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	user := &TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user}

	stableKey := traceInstance.getStableObjectKey(user)

	// Mock 缓存查询返回错误
	mockParamRepo.On("FindParamCacheByAddr", stableKey).Return((*model.ParamCache)(nil), assert.AnError)

	// Mock 保存参数
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1001), nil)

	// 执行测试
	traceInstance.DealPointerMethod(123, params)

	// 验证调用
	mockParamRepo.AssertExpectations(t)

	// 验证SaveParam被调用时存储了完整数据（因为缓存查询失败）
	calls := mockParamRepo.Calls
	var saveCall *mock.Call
	for _, call := range calls {
		if call.Method == "SaveParam" {
			saveCall = &call
			break
		}
	}
	assert.NotNil(t, saveCall)

	paramData := saveCall.Arguments[0].(*model.ParamStoreData)
	assert.Equal(t, int64(0), paramData.BaseID) // 应该是0，因为没有缓存

	// 验证没有调用SaveParamCache（因为缓存查询失败）
	mockParamRepo.AssertNotCalled(t, "SaveParamCache")
}

// TestDealPointerMethod_JSONPatchError 测试JSON Patch创建失败
func TestDealPointerMethod_JSONPatchError(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	user := &TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user}

	stableKey := traceInstance.getStableObjectKey(user)

	// 创建无效的缓存数据（无法解析为JSON）
	invalidData := []byte("invalid json data")
	existingCache := &model.ParamCache{
		Addr:   stableKey,
		BaseID: 500,
		Data:   append(magicNumber, invalidData...),
	}

	// Mock 缓存查询返回存在的缓存
	mockParamRepo.On("FindParamCacheByAddr", stableKey).Return(existingCache, nil)

	// Mock 保存参数
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1001), nil)

	// 执行测试
	traceInstance.DealPointerMethod(123, params)

	// 验证调用
	mockParamRepo.AssertExpectations(t)

	// 验证SaveParam被调用时存储了完整数据（因为JSON patch失败）
	calls := mockParamRepo.Calls
	var saveCall *mock.Call
	for _, call := range calls {
		if call.Method == "SaveParam" {
			saveCall = &call
			break
		}
	}
	assert.NotNil(t, saveCall)

	paramData := saveCall.Arguments[0].(*model.ParamStoreData)
	assert.Equal(t, int64(0), paramData.BaseID) // 应该是0，因为使用了完整数据
}

// TestDealPointerMethod_MultipleParams 测试多个参数
func TestDealPointerMethod_MultipleParams(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	user := &TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user, "arg1", 42, true}

	stableKey := traceInstance.getStableObjectKey(user)

	// Mock 缓存查询返回nil（首次遇到）
	mockParamRepo.On("FindParamCacheByAddr", stableKey).Return((*model.ParamCache)(nil), nil)

	// Mock 保存参数：1个接收者 + 3个其他参数 = 4次调用
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1001), nil).Times(4)
	mockParamRepo.On("SaveParamCache", mock.AnythingOfType("*model.ParamCache")).Return(int64(1), nil)

	// 执行测试
	traceInstance.DealPointerMethod(123, params)

	// 验证所有调用
	mockParamRepo.AssertExpectations(t)

	// 验证参数位置正确
	calls := mockParamRepo.Calls
	saveParamCalls := make([]*mock.Call, 0)
	for i := range calls {
		if calls[i].Method == "SaveParam" {
			saveParamCalls = append(saveParamCalls, &calls[i])
		}
	}

	assert.Len(t, saveParamCalls, 4)

	// 验证接收者参数
	receiverParam := saveParamCalls[0].Arguments[0].(*model.ParamStoreData)
	assert.Equal(t, 0, receiverParam.Position)
	assert.True(t, receiverParam.IsReceiver)

	// 验证其他参数的位置
	for i := 1; i < 4; i++ {
		param := saveParamCalls[i].Arguments[0].(*model.ParamStoreData)
		assert.Equal(t, i, param.Position)
		assert.False(t, param.IsReceiver)
	}
}

// TestDealPointerMethod_SaveCacheError 测试保存缓存失败
func TestDealPointerMethod_SaveCacheError(t *testing.T) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	user := &TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user}

	stableKey := traceInstance.getStableObjectKey(user)

	// Mock 缓存查询返回nil（首次遇到）
	mockParamRepo.On("FindParamCacheByAddr", stableKey).Return((*model.ParamCache)(nil), nil)

	// Mock 保存参数成功，但保存缓存失败
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1001), nil)
	mockParamRepo.On("SaveParamCache", mock.AnythingOfType("*model.ParamCache")).Return(int64(0), assert.AnError)

	// 执行测试（应该不会panic，只会记录警告）
	assert.NotPanics(t, func() {
		traceInstance.DealPointerMethod(123, params)
	})

	// 验证所有调用
	mockParamRepo.AssertExpectations(t)
}

// TestGetStableObjectKey 测试稳定对象键生成
func TestGetStableObjectKey(t *testing.T) {
	traceInstance, _ := setupTestTraceInstance()

	// 测试指针类型
	user := &TestUser{ID: 1, Name: "test", Age: 25}
	key1 := traceInstance.getStableObjectKey(user)
	assert.NotEmpty(t, key1)
	assert.Contains(t, key1, "TestUser@")

	// 同一个对象应该生成相同的键
	key2 := traceInstance.getStableObjectKey(user)
	assert.Equal(t, key1, key2)

	// 不同对象应该生成不同的键
	user2 := &TestUser{ID: 2, Name: "test2", Age: 30}
	key3 := traceInstance.getStableObjectKey(user2)
	assert.NotEqual(t, key1, key3)
	assert.Contains(t, key3, "TestUser@")

	// 测试非指针类型
	userValue := TestUser{ID: 1, Name: "test", Age: 25}
	key4 := traceInstance.getStableObjectKey(userValue)
	assert.Empty(t, key4)
}

// BenchmarkDealPointerMethod 性能测试
func BenchmarkDealPointerMethod(b *testing.B) {
	traceInstance, mockParamRepo := setupTestTraceInstance()

	user := &TestUser{ID: 1, Name: "test", Age: 25}
	params := []interface{}{user, "arg1", 42}

	stableKey := traceInstance.getStableObjectKey(user)

	// Mock设置
	mockParamRepo.On("FindParamCacheByAddr", stableKey).Return((*model.ParamCache)(nil), nil)
	mockParamRepo.On("SaveParam", mock.AnythingOfType("*model.ParamStoreData")).Return(int64(1001), nil)
	mockParamRepo.On("SaveParamCache", mock.AnythingOfType("*model.ParamCache")).Return(int64(1), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		traceInstance.DealPointerMethod(int64(i), params)
	}
}

// TestCompressDecompress 测试压缩解压功能
func TestCompressDecompress(t *testing.T) {
	testData := `{"id":1,"name":"test user","age":25,"details":{"address":"123 Main St","phone":"555-1234"}}`

	// 测试压缩
	compressed := compress(testData)
	assert.NotEmpty(t, compressed)
	assert.True(t, len(compressed) > len(magicNumber)) // 应该包含魔数

	// 验证魔数
	assert.Equal(t, magicNumber, compressed[:len(magicNumber)])

	// 测试解压
	decompressed := decompress(compressed)
	assert.Equal(t, testData, decompressed)

	// 测试无魔数的数据（向后兼容）
	legacyData := []byte("legacy uncompressed data")
	decompressedLegacy := decompress(legacyData)
	assert.Equal(t, "legacy uncompressed data", decompressedLegacy)
}
