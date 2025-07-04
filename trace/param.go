package trace

import (
	"encoding/json"
	"fmt"
	"reflect"

	go_spew "github.com/davecgh/go-spew/spew"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
	"github.com/toheart/functrace/spew"
)

// DealNormalMethod 处理普通方法的参数
func (t *TraceInstance) DealNormalMethod(traceID int64, params []interface{}) {
	for i, item := range params {
		paramStoreData := &model.ParamStoreData{
			ID:       t.gParamId.Add(1),
			TraceID:  traceID,
			Position: i,
			Data:     spew.Sdump(item),
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
			Data:     spew.Sdump(item),
		}
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    paramStoreData,
		})
	}
}

func (t *TraceInstance) GetParamFromCache(addr string) *receiverInfo {
	// 从数据库查询参数缓存
	cache, err := repositoryFactory.GetParamRepository().FindParamCacheByAddr(addr)
	if err != nil {
		t.log.WithFields(logrus.Fields{
			"error": err,
			"addr":  addr,
		}).Error("failed to get param from database cache")
		return nil
	}

	if cache == nil {
		return nil
	}

	return &receiverInfo{
		TraceID: cache.TraceID,
		BaseID:  cache.BaseID,
		Data:    cache.Data,
	}
}

func (t *TraceInstance) SetParamToCache(addr string, info *receiverInfo) {
	// 创建参数缓存对象
	cache := &model.ParamCache{
		Addr:    addr,
		TraceID: info.TraceID,
		BaseID:  info.BaseID,
		Data:    info.Data,
	}

	// 保存到数据库
	_, err := repositoryFactory.GetParamRepository().SaveParamCache(cache)
	if err != nil {
		t.log.WithFields(logrus.Fields{
			"error": err,
			"addr":  addr,
			"cache": cache,
		}).Error("failed to save param to database cache")
	}
}

func (t *TraceInstance) GetAddrKey(receiver interface{}) string {
	return fmt.Sprintf("%d", reflect.ValueOf(receiver).Pointer())
}

// DealPointerMethod 处理指针方法的参数
func (t *TraceInstance) DealPointerMethod(traceID int64, params []interface{}) {
	if len(params) == 0 {
		return
	}
	// 获取第一个参数的地址（接收者）
	receiver := params[0]
	addr := t.GetAddrKey(receiver)
	t.log.WithFields(logrus.Fields{
		"addr": addr,
	}).Info("DealPointerMethod")
	// 处理第一个参数(接收者)
	receiverData := spew.Sdump(receiver)

	paramStoreData := &model.ParamStoreData{
		ID:         t.gParamId.Add(1),
		TraceID:    traceID,
		Position:   0,
		IsReceiver: true,
	}

	// 检查缓存中是否存在该接收者
	info := t.GetParamFromCache(addr)
	go_spew.Config.MaxDepth = 2
	if info != nil {
		// 存在则计算差异
		if diff, err := createJSONPatch(info.Data, receiverData); err != nil {
			t.log.WithFields(logrus.Fields{
				"error":        err,
				"info":         info,
				"receiverData": spew.Sdump(receiver),
				"receiver":     go_spew.Sdump(receiver),
			}).Error("failed to create json patch")
			paramStoreData.Data = receiverData
			panic(err)
		} else {
			paramStoreData.Data = diff
			paramStoreData.BaseID = info.BaseID
		}
	} else {
		// 不存在则存储完整数据
		paramStoreData.Data = receiverData
		t.SetParamToCache(addr, &receiverInfo{
			TraceID: traceID,
			BaseID:  paramStoreData.ID,
			Data:    receiverData,
		})
	}
	t.sendOp(&DataOp{
		OpType: OpTypeInsert,
		Arg:    paramStoreData,
	})

	// 处理其他参数
	for i := 1; i < len(params); i++ {
		paramStoreData := &model.ParamStoreData{
			ID:       t.gParamId.Add(1),
			TraceID:  traceID,
			Position: i,
			Data:     spew.Sdump(params[i]),
		}
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    paramStoreData,
		})
	}
}

// storeParam 存储参数数据
func (t *TraceInstance) storeParam(param *model.ParamStoreData) {
	_, err := repositoryFactory.GetParamRepository().SaveParam(param)
	if err != nil {
		t.log.WithFields(logrus.Fields{
			"error": err,
			"param": param,
		}).Error("save param failed")
	}
}

// createJSONPatch 创建JSON差异
func createJSONPatch(original, modified string) (string, error) {
	// 先做深拷贝，避免patch过程中结构体内容变化导致panic
	var origObj, modObj interface{}
	if err := json.Unmarshal([]byte(original), &origObj); err != nil {
		return "", fmt.Errorf("can't unmarshal original: %w, original: %s", err, original)
	}
	if err := json.Unmarshal([]byte(modified), &modObj); err != nil {
		return "", fmt.Errorf("can't unmarshal modified: %w, modified: %s", err, modified)
	}
	origCopy, _ := json.Marshal(origObj)
	modCopy, _ := json.Marshal(modObj)
	// 创建差异
	patch, err := jsonpatch.CreateMergePatch(origCopy, modCopy)
	if err != nil {
		return "", fmt.Errorf("can't create json patch: %w", err)
	}

	// 如果差异为空对象，返回空
	if string(patch) == "" {
		return "{}", nil
	}

	return string(patch), nil
}

func (t *TraceInstance) DeleteParamFromCache(traceID int64) {
	// 使用数据库删除参数缓存
	err := repositoryFactory.GetParamRepository().DeleteParamCacheByTraceID(traceID)
	if err != nil {
		t.log.WithFields(logrus.Fields{
			"error":   err,
			"traceID": traceID,
		}).Error("failed to delete param from database cache")
	}
}
