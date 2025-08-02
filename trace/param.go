package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
	objDump "github.com/toheart/functrace/objectdump"
)

var (
	zstdEncoder, _ = zstd.NewWriter(nil)
	zstdDecoder, _ = zstd.NewReader(nil)
	magicNumber    = []byte{'F', 'T', 'Z', '$'} // Fun-Trace-Zstd Magic Number
)

// compress uses zstd to compress a string and returns the compressed data prefixed with a magic number.
func compress(s string) []byte {
	compressed := zstdEncoder.EncodeAll([]byte(s), nil)
	return append(magicNumber, compressed...)
}

// decompress uses zstd to decompress data. It checks for a magic number to identify
// compressed data and includes a fallback for uncompressed or corrupted data.
func decompress(data []byte) string {
	if len(data) < len(magicNumber) || !bytes.Equal(data[:len(magicNumber)], magicNumber) {
		// Data doesn't have the magic number, so it's legacy uncompressed data.
		return string(data)
	}

	// Data has the magic number, so it should be decompressed.
	decompressed, err := zstdDecoder.DecodeAll(data[len(magicNumber):], nil)
	if err != nil {
		// Data is corrupted. The calling function will fail to unmarshal it,
		// which will be logged as a high-level error.
		return string(data)
	}
	return string(decompressed)
}

// DealNormalMethod 处理普通方法的参数
func (t *TraceInstance) DealNormalMethod(traceID int64, params []interface{}) {
	t.safeExecute(func() {
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
	})
}

// DealValueMethod 处理值方法的参数
func (t *TraceInstance) DealValueMethod(traceID int64, params []interface{}) {
	t.safeExecute(func() {
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
	})
}

func (t *TraceInstance) GetAddrKey(receiver interface{}) string {
	// Ensure the receiver is a pointer, as only pointers can have finalizers.
	val := reflect.ValueOf(receiver)
	if val.Kind() != reflect.Ptr {
		return ""
	}
	return fmt.Sprintf("%d", val.Pointer())
}

// DealPointerMethod 处理指针方法的参数
func (t *TraceInstance) DealPointerMethod(traceID int64, params []interface{}) {
	t.safeExecute(func() {
		if len(params) == 0 {
			return
		}
		// 获取第一个参数的地址（接收者）
		receiver := params[0]
		addr := t.GetAddrKey(receiver)
		if addr == "" {
			// Not a pointer, cannot be cached. Treat as a normal method.
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

		// Lock the entire read-modify-write operation for the cache
		t.paramCacheLock.Lock()
		defer t.paramCacheLock.Unlock()

		// 检查缓存中是否存在该接收者
		cache, err := repositoryFactory.GetParamRepository().FindParamCacheByAddr(addr)
		if err != nil {
			t.log.WithFields(logrus.Fields{"error": err, "addr": addr}).Error("failed to get param from database cache")
			paramStoreData.Data = compress(receiverDataStr)
		} else if cache != nil {
			// 存在则解压旧数据并计算差异
			originalDataStr := decompress(cache.Data)
			if diff, err := createJSONPatch(originalDataStr, receiverDataStr); err != nil {
				t.log.WithFields(logrus.Fields{"error": err}).Error("failed to create json patch, storing full data")
				paramStoreData.Data = compress(receiverDataStr)
			} else {
				paramStoreData.Data = compress(diff)
				paramStoreData.BaseID = cache.BaseID
			}
			// 只要访问，就更新TTL
			t.ttlManager.Update(addr)
		} else {
			// 不存在则存储完整数据
			compressedData := compress(receiverDataStr)
			paramStoreData.Data = compressedData

			newCache := &model.ParamCache{
				Addr:   addr,
				BaseID: paramStoreData.ID,
				Data:   compressedData,
			}
			if _, err := repositoryFactory.GetParamRepository().SaveParamCache(newCache); err != nil {
				t.log.WithFields(logrus.Fields{"error": err, "cache": newCache}).Error("failed to save param to database cache")
			} else {
				// 更新TTL缓存
				t.ttlManager.Update(addr)
			}
		}

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
	})
}

// safeSetFinalizer wraps runtime.SetFinalizer with a recover mechanism to prevent panics.
func (t *TraceInstance) safeSetFinalizer(obj interface{}) {
	defer func() {
		if r := recover(); r != nil {
			t.log.WithFields(logrus.Fields{
				"error": r,
				"type":  fmt.Sprintf("%T", obj),
				"addr":  t.GetAddrKey(obj),
			}).Error("runtime.SetFinalizer failed. This is likely because the traced method's receiver is a pointer to a field within another struct, not a pointer to a separately allocated object. Caching for this object will not be automatically cleaned up by the garbage collector.")
		}
	}()
	runtime.SetFinalizer(obj, t.finalizer)
}

// finalizer is the function called by the Go runtime when a cached object is garbage collected.
func (t *TraceInstance) finalizer(obj interface{}) {
	t.safeExecute(func() {
		addr := t.GetAddrKey(obj)
		if addr == "" {
			return // Should not happen if SetFinalizer was called correctly
		}

		t.paramCacheLock.Lock()
		defer t.paramCacheLock.Unlock()

		err := repositoryFactory.GetParamRepository().DeleteParamCacheByAddr(addr)
		if err != nil {
			t.log.WithFields(logrus.Fields{"error": err, "addr": addr}).Error("failed to delete param cache in finalizer")
		} else {
			t.log.WithFields(logrus.Fields{"addr": addr}).Info("successfully deleted param cache in finalizer")
		}
	})
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
