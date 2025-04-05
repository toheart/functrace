package trace

import (
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
)

// DealNormalMethod 处理普通方法的参数
func (t *TraceInstance) DealNormalMethod(traceID int64, params []interface{}) {
	for i, item := range params {
		paramStoreData := &model.ParamStoreData{
			ID:       t.gParamId.Add(1),
			TraceID:  traceID,
			Position: i,
			Data:     t.spewConfig.Sdump(item),
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
			Data:     t.spewConfig.Sdump(item),
		}
		t.sendOp(&DataOp{
			OpType: OpTypeInsert,
			Arg:    paramStoreData,
		})
	}
}

func (t *TraceInstance) GetParamFromCache(addr string) *receiverInfo {
	t.RLock()
	defer t.RUnlock()
	if info, exists := t.paramCache[addr]; exists {
		return info
	}
	return nil
}

func (t *TraceInstance) SetParamToCache(addr string, info *receiverInfo) {
	t.Lock()
	defer t.Unlock()
	t.paramCache[addr] = info
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
	receiverData := t.spewConfig.Sdump(receiver)
	paramStoreData := &model.ParamStoreData{
		ID:         t.gParamId.Add(1),
		TraceID:    traceID,
		Position:   0,
		IsReceiver: true,
	}

	// 检查缓存中是否存在该接收者
	info := t.GetParamFromCache(addr)
	if info != nil {
		// 存在则计算差异
		if diff, err := createJSONPatch(info.Data, receiverData); err != nil {
			paramStoreData.Data = receiverData
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
			Data:     t.spewConfig.Sdump(params[i]),
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
	// 创建差异
	patch, err := jsonpatch.CreateMergePatch([]byte(original), []byte(modified))
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
	t.RLock()
	tmp := make(map[string]*receiverInfo)
	for k, v := range t.paramCache {
		tmp[k] = v
	}
	t.RUnlock()

	// 获取key
	delKey := ""
	for k, v := range tmp {
		if v.TraceID == traceID {
			delKey = k
			break
		}
	}

	if delKey != "" {
		t.Lock()
		delete(t.paramCache, delKey)
		t.Unlock()
	}
}
