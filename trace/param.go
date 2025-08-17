package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
	objDump "github.com/toheart/functrace/objectdump"
)

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

// DealNormalMethod 处理普通方法的参数
func (t *TraceInstance) DealNormalMethod(traceID int64, params []interface{}) {
	for i, item := range params {
		paramStoreData := &model.ParamStoreData{
			ID:       t.gParamId.Add(1),
			TraceID:  traceID,
			Position: i,
			Data:     compress(objDump.Sdump(item)),
		}
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    paramStoreData,
		})
	}
}

// DealValueMethod 处理值方法的参数
func (t *TraceInstance) DealValueMethod(traceID int64, params []interface{}) {
	for i, item := range params {
		paramStoreData := &model.ParamStoreData{
			ID:       t.gParamId.Add(1),
			TraceID:  traceID,
			Position: i,
			Data:     compress(objDump.Sdump(item)),
		}
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    paramStoreData,
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

	receiverDataStr := objDump.Sdump(receiver)

	paramStoreData := &model.ParamStoreData{
		ID:         t.gParamId.Add(1),
		TraceID:    traceID,
		Position:   0,
		IsReceiver: true,
	}

	// 先在锁外进行数据库查询，避免长时间持有锁
	cache, err := repositoryFactory.GetParamRepository().FindParamCacheByAddr(stableKey)

	// 在锁内处理缓存逻辑，进行快速的内存操作
	t.paramCacheLock.Lock()
	needCreateCache := false

	if err != nil {
		// 数据库查询失败，记录错误并存储完整数据
		t.log.WithFields(logrus.Fields{"error": err, "stableKey": stableKey}).Error("failed to get param from database cache")
		paramStoreData.Data = compress(receiverDataStr)
	} else if cache != nil {
		// 缓存存在，计算差异
		originalDataStr := decompress(cache.Data)
		if diff, err := createJSONPatch(originalDataStr, receiverDataStr); err != nil {
			// diff计算失败，存储完整数据（实用主义：不影响主流程）
			t.log.WithFields(logrus.Fields{"error": err, "stableKey": stableKey}).Warn("failed to create json patch, storing full data")
			paramStoreData.Data = compress(receiverDataStr)
		} else {
			// 成功计算diff，存储压缩数据
			paramStoreData.Data = compress(diff)
			paramStoreData.BaseID = cache.BaseID
		}
	} else {
		// 首次遇到这个对象，存储完整数据并标记需要创建缓存
		compressedData := compress(receiverDataStr)
		paramStoreData.Data = compressedData
		needCreateCache = true
	}
	t.paramCacheLock.Unlock()

	// 在锁外进行数据库操作（如果需要创建新缓存）
	if needCreateCache {
		newCache := &model.ParamCache{
			Addr:   stableKey, // 使用稳定的对象键
			BaseID: paramStoreData.ID,
			Data:   paramStoreData.Data,
		}
		// 使用INSERT OR REPLACE避免并发插入冲突
		if _, err := repositoryFactory.GetParamRepository().SaveParamCache(newCache); err != nil {
			// 缓存保存失败不影响主流程，仅记录警告
			t.log.WithFields(logrus.Fields{"error": err, "stableKey": stableKey}).Warn("failed to save param cache")
		}
	}
	// 只要访问，就更新TTL

	t.sendOp(&DataOp{
		OpType: OpTypeInsert,
		Arg:    paramStoreData,
	})

	// 处理其他参数 (这部分不需要在锁内)
	for i := 1; i < len(params); i++ {
		otherParamData := &model.ParamStoreData{
			ID:       t.gParamId.Add(1),
			TraceID:  traceID,
			Position: i,
			Data:     compress(objDump.Sdump(params[i])),
		}
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    otherParamData,
		})
	}
}

// storeParam 存储参数数据
func (t *TraceInstance) storeParam(param *model.ParamStoreData) {
	t.safeExecute(func() {
		_, err := repositoryFactory.GetParamRepository().SaveParam(param)
		if err != nil {
			t.log.WithFields(logrus.Fields{"error": err, "param": param}).Error("save param failed")
		}
	})
}

// createJSONPatch 创建JSON差异
func createJSONPatch(original, modified string) (string, error) {
	var origObj, modObj interface{}
	if err := json.Unmarshal([]byte(original), &origObj); err != nil {
		return "", fmt.Errorf("can't unmarshal original: %w, original: %s", err, original)
	}
	if err := json.Unmarshal([]byte(modified), &modObj); err != nil {
		return "", fmt.Errorf("can't unmarshal modified: %w, modified: %s", err, modified)
	}
	origCopy, err := json.Marshal(origObj)
	if err != nil {
		return "", fmt.Errorf("can't marshal original copy: %w", err)
	}
	modCopy, err := json.Marshal(modObj)
	if err != nil {
		return "", fmt.Errorf("can't marshal modified copy: %w", err)
	}
	patch, err := jsonpatch.CreateMergePatch(origCopy, modCopy)
	if err != nil {
		return "", fmt.Errorf("can't create json patch: %w", err)
	}
	if string(patch) == "" {
		return "{}", nil
	}
	return string(patch), nil
}
