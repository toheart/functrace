package trace

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
	objDump "github.com/toheart/functrace/objectdump"
)

// nextParamID 使用分片ID生成器生成全局唯一的参数ID
func (t *TraceInstance) nextParamID(shardKey uint64) int64 {
	if t.idGen != nil {
		return t.idGen.NextParamID(shardKey)
	}
	// 回退到全局原子，确保向后兼容
	return t.gParamId.Add(1)
}

// idHint 为参数ID分片选择提供一个简单的键
func idHint(params []interface{}, index int) uint64 {
	// 以参数位置作为简易分片键，避免所有参数都打到同一分片
	return uint64(index + 1)
}

var (
	magicNumber = []byte{'F', 'T', 'Z', '$'} // Fun-Trace-Zstd Magic Number
)

// compress uses zstd to compress a string and returns the compressed data prefixed with a magic number.
// 为了线程安全，每次调用都创建新的编码器
func compress(s string) []byte {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		// 如果创建编码器失败，返回未压缩数据
		return append(magicNumber, []byte(s)...)
	}
	defer encoder.Close()

	compressed := encoder.EncodeAll([]byte(s), nil)
	return append(magicNumber, compressed...)
}

// decompress uses zstd to decompress data. It checks for a magic number to identify
// compressed data and includes a fallback for uncompressed or corrupted data.
// 为了线程安全，每次调用都创建新的解码器
func decompress(data []byte) string {
	if len(data) < len(magicNumber) || !bytes.Equal(data[:len(magicNumber)], magicNumber) {
		// Data doesn't have the magic number, so it's legacy uncompressed data.
		return string(data)
	}

	// Data has the magic number, so it should be decompressed.
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		// 如果创建解码器失败，返回原始数据
		return string(data)
	}
	defer decoder.Close()

	decompressed, err := decoder.DecodeAll(data[len(magicNumber):], nil)
	if err != nil {
		// Data is corrupted. The calling function will fail to unmarshal it,
		// which will be logged as a high-level error.
		return string(data)
	}
	return string(decompressed)
}

// sdumpSafe 安全地对对象进行转储，避免 objectdump 在 unsafe 反射时触发 checkptr panic
func sdumpSafe(v interface{}) (s string) {
	defer func() {
		if r := recover(); r != nil {
			// 发生 panic 时回退到类型名，避免崩溃
			s = fmt.Sprintf("%T", v)
		}
	}()
	return objDump.Sdump(v)
}

// DealNormalMethod 处理普通方法的参数
func (t *TraceInstance) DealNormalMethod(traceID int64, params []interface{}) {
	for i, item := range params {
		// 下沉到后台处理，减轻热路径负担
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    &processParamTask{TraceID: traceID, Position: i, Value: item},
		})
	}
}

// DealValueMethod 处理值方法的参数
func (t *TraceInstance) DealValueMethod(traceID int64, params []interface{}) {
	for i, item := range params {
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    &processParamTask{TraceID: traceID, Position: i, Value: item},
		})
	}
}

func (t *TraceInstance) GetAddrKey(receiver interface{}) string {
	// 确保接收者是指针类型，以便获取其地址
	val := reflect.ValueOf(receiver)
	if val.Kind() != reflect.Ptr {
		return ""
	}
	return fmt.Sprintf("%d", val.Pointer())
}

// getStableObjectKey 生成稳定的对象键：包名.类型名@指针地址
func (t *TraceInstance) getStableObjectKey(receiver interface{}) string {
	val := reflect.ValueOf(receiver)
	if val.Kind() != reflect.Ptr {
		return ""
	}

	// 检查是否为nil指针
	if val.IsNil() {
		return ""
	}

	// 获取类型信息（包含包名）
	elemType := val.Elem().Type()
	pkgPath := elemType.PkgPath()
	typeName := elemType.Name()

	// 处理匿名类型或内置类型
	if pkgPath == "" {
		pkgPath = "builtin"
	}
	if typeName == "" {
		// 对于匿名类型，使用简化的类型描述
		typeName = t.simplifyTypeName(elemType)
	}

	// 组合：包名.类型名@指针地址
	addr := val.Pointer()
	return fmt.Sprintf("%s.%s@%d", pkgPath, typeName, addr)
}

// simplifyTypeName 简化类型名称，避免过长的键和特殊字符
func (t *TraceInstance) simplifyTypeName(elemType reflect.Type) string {
	typeStr := elemType.String()

	// 处理各种特殊情况
	switch elemType.Kind() {
	case reflect.Struct:
		// 匿名结构体使用简化名称
		if elemType.Name() == "" {
			return "anonymous_struct"
		}
		return elemType.Name()
	case reflect.Slice:
		return "slice"
	case reflect.Map:
		return "map"
	case reflect.Array:
		return "array"
	case reflect.Chan:
		return "chan"
	case reflect.Func:
		return "func"
	case reflect.Interface:
		return "interface"
	default:
		// 移除特殊字符，避免数据库查询问题
		// 替换 []{}()<> 等字符为下划线
		simplified := typeStr
		specialChars := []string{"[", "]", "{", "}", "(", ")", "<", ">", " ", ";"}
		for _, char := range specialChars {
			simplified = strings.ReplaceAll(simplified, char, "_")
		}
		// 限制长度
		return simplified
	}
}

// DealPointerMethod 处理指针方法的参数
func (t *TraceInstance) DealPointerMethod(traceID int64, params []interface{}) {
	if len(params) == 0 {
		return
	}
	// 获取第一个参数（接收者）
	receiver := params[0]
	stableKey := t.getStableObjectKey(receiver)
	if stableKey == "" {
		// 不是指针类型，回退到普通方法处理
		t.DealNormalMethod(traceID, params)
		return
	}

	// 接收者下沉到后台任务
	t.sendOp(&DataOp{
		OpType: OpTypeInsert,
		Arg:    &processPointerReceiverTask{TraceID: traceID, StableKey: stableKey, Receiver: receiver},
	})

	// 其他参数（从索引1开始）
	for i := 1; i < len(params); i++ {
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    &processParamTask{TraceID: traceID, Position: i, Value: params[i]},
		})
	}
}

// storeParam 存储参数数据
func (t *TraceInstance) storeParam(param *model.ParamStoreData) {
	t.safeExecute(func() {
		// 交给批量器处理；若已关闭或通道不可用，则直接降级为单条写入，避免 panic/丢失
		if t.closedFlag.Load() {
			if _, err := repositoryFactory.GetParamRepository().SaveParam(param); err != nil {
				t.log.WithFields(logrus.Fields{"error": err, "param": param}).Error("save param failed")
			}
			return
		}
		// 迁移后批量器由 ParamPipeline 管理，storeParam 仅做降级直写
		if _, err := repositoryFactory.GetParamRepository().SaveParam(param); err != nil {
			t.log.WithFields(logrus.Fields{"error": err, "param": param}).Error("save param failed")
		}
	})
}

// createJSONPatch 创建JSON差异
func createJSONPatch(original, modified string) (string, error) {
	if original == modified {
		return "{}", nil
	}
	patch, err := jsonpatch.CreateMergePatch([]byte(original), []byte(modified))
	if err != nil {
		return "", fmt.Errorf("can't create json patch: %w", err)
	}
	if len(patch) == 0 {
		return "{}", nil
	}
	return string(patch), nil
}

// 后台参数处理任务类型
type processParamTask struct {
	TraceID  int64
	Position int
	Value    interface{}
}

type processPointerReceiverTask struct {
	TraceID   int64
	StableKey string
	Receiver  interface{}
}

// 处理普通/值参数的后台任务
func (t *TraceInstance) handleProcessParamTask(task *processParamTask) {
	dumped := sdumpSafe(task.Value)
	data := compress(dumped)
	paramStoreData := &model.ParamStoreData{
		ID:       t.nextParamID(uint64(task.Position + 1)),
		TraceID:  task.TraceID,
		Position: task.Position,
		Data:     data,
	}
	if t.pipelines != nil {
		t.pipelines.Param.Enqueue(paramStoreData)
		return
	}
	t.storeParam(paramStoreData)
}

// 处理指针接收者的后台任务（含差异与缓存）
func (t *TraceInstance) handleProcessPointerReceiverTask(task *processPointerReceiverTask) {
	receiverDataStr := sdumpSafe(task.Receiver)

	paramStoreData := &model.ParamStoreData{
		ID:         t.nextParamID(0),
		TraceID:    task.TraceID,
		Position:   0,
		IsReceiver: true,
	}

	// 先在锁外进行数据库查询（singleflight 合并）
	var (
		cache *model.ParamCache
		err   error
	)
	key := "recv:" + task.StableKey
	_, _, _ = t.recvSFG.Do(key, func() (interface{}, error) {
		cache, err = repositoryFactory.GetParamRepository().FindParamCacheByAddr(task.StableKey)
		return nil, nil
	})

	// 在锁内处理内存状态
	t.paramCacheLock.Lock()
	needCreateCache := false

	if err != nil {
		t.log.WithFields(logrus.Fields{"error": err, "stableKey": task.StableKey}).Error("failed to get param from database cache")
		paramStoreData.Data = compress(receiverDataStr)
	} else if cache != nil {
		originalDataStr := decompress(cache.Data)
		if diff, err := createJSONPatch(originalDataStr, receiverDataStr); err != nil {
			t.log.WithFields(logrus.Fields{"error": err, "stableKey": task.StableKey}).Warn("failed to create json patch, storing full data")
			paramStoreData.Data = compress(receiverDataStr)
		} else {
			paramStoreData.Data = compress(diff)
			paramStoreData.BaseID = cache.BaseID
		}
	} else {
		// 首次遇到这个对象
		compressedData := compress(receiverDataStr)
		paramStoreData.Data = compressedData
		needCreateCache = true
	}
	t.paramCacheLock.Unlock()

	if needCreateCache {
		newCache := &model.ParamCache{
			Addr:   task.StableKey,
			BaseID: paramStoreData.ID,
			Data:   paramStoreData.Data,
		}
		_, _, _ = t.recvSFG.Do(key+":save", func() (interface{}, error) {
			if _, err := repositoryFactory.GetParamRepository().SaveParamCache(newCache); err != nil {
				t.log.WithFields(logrus.Fields{"error": err, "stableKey": task.StableKey}).Warn("failed to save param cache")
			}
			return nil, nil
		})
	}

	if t.pipelines != nil {
		t.pipelines.Param.Enqueue(paramStoreData)
		return
	}
	t.storeParam(paramStoreData)
}
