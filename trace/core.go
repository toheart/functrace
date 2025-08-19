package trace

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain/model"
)

// enterTrace 记录函数调用的开始并存储必要的跟踪详情
func (t *TraceInstance) EnterTrace(id uint64, name string, params []interface{}) (*model.TraceData, time.Time) {
	startTime := time.Now() // 记录开始时间
	// 通过会话独享状态准备进入信息
	session := t.sessions.GetOrCreate(id)
	// 确保会话转发器已启动
	session.EnsureForwarder(t)
	indent, parentId, traceId := session.PrepareEnter(t)
	// 格式化时间序列，保留2位小数
	duration := time.Since(currentNow)
	seq := fmt.Sprintf("%.2f", float64(duration.Milliseconds())/1000.0)
	// 创建跟踪数据
	traceData := &model.TraceData{
		ID:        traceId,
		Name:      name,
		GID:       id,
		Indent:    indent,
		ParentId:  parentId,
		CreatedAt: startTime.Format(TimeFormat),
		Seq:       seq,
	}

	// 根据参数存储模式决定是否处理参数
	funcInfo := t.isStructMethod(name)
	traceData.MethodType = funcInfo.Type

	// 记录原始参数数量，避免在处理过程中被修改
	originalParamsCount := len(params)

	if t.config.ParamStoreMode != ParamStoreModeNone {
		switch funcInfo.Type {
		case MethodTypeNormal:
			t.DealNormalMethod(traceId, params)
		case MethodTypeValue:
			t.DealValueMethod(traceId, params)
		case MethodTypePointer:
			if t.config.ParamStoreMode == ParamStoreModeNormal {
				// 普通模式下，跳过第一个参数（接收者）
				if len(params) > 0 {
					t.DealNormalMethod(traceId, params[1:])
				}
			} else {
				t.DealPointerMethod(traceId, params)
			}
		}
	}
	traceData.ParamsCount = originalParamsCount
	session.Enqueue(&DataOp{
		OpType: OpTypeInsert,
		Arg:    traceData,
	})
	// 记录日志
	t.logFunctionEntry(id, name, indent, parentId, len(params), startTime)
	return traceData, startTime
}

// ExitTrace 记录函数调用的结束并减少跟踪缩进
func (t *TraceInstance) ExitTrace(info *GoroutineInfo, traceData *model.TraceData, startTime time.Time) {
	// 计算函数执行时间（无论是否出错都要记录）
	duration := time.Since(startTime)

	// 更新跟踪信息
	indent := t.updateTraceIndent(info.ID)
	logIndent := indent
	if indent < 0 {
		// 如果更新缩进失败，使用默认值继续处理，确保数据完整性
		t.log.WithFields(logrus.Fields{
			"goroutine": info.ID,
			"function":  traceData.Name,
		}).Warn("failed to update trace indent, using default value")
		logIndent = 0
	}

	// 更新函数执行时间和完成状态
	t.sendOp(&DataOp{
		OpType: OpTypeUpdate,
		Arg: &model.TraceData{
			ID:         traceData.ID,
			TimeCost:   duration.String(),
			IsFinished: 1,
		},
	})

	// 记录日志
	t.logFunctionExit(info.ID, traceData.Name, logIndent, duration.String())

	// 检查是否是main.main函数退出，如果是则等待所有数据入库完成
	if t.isMainFunction(traceData.Name) {
		t.log.WithFields(logrus.Fields{
			"function": traceData.Name,
			"indent":   logIndent,
		}).Info("main.main function exiting, ensuring all data is persisted before exit")

		// 直接关闭trace实例，这会自动等待所有数据入库完成
		if err := t.Close(); err != nil {
			t.log.WithFields(logrus.Fields{"error": err}).Error("failed to close trace instance")
		} else {
			t.log.Info("trace instance closed successfully, all data has been persisted")
		}
	}
}

// updateTraceIndent 更新跟踪缩进并返回当前缩进级别
func (t *TraceInstance) updateTraceIndent(id uint64) int {
	// 会话内回退
	session := t.sessions.GetOrCreate(id)
	return session.OnExit()
}

// logFunctionEntry 记录函数进入的日志
func (t *TraceInstance) logFunctionEntry(gid uint64, name string, indent int, parentId int64, paramCount int, startTime time.Time) {
	// 防止负数导致 panic
	indentCount := indent
	if indentCount < 0 {
		indentCount = 0
	}
	indentStr := strings.Repeat("  ", indentCount)
	t.log.WithFields(logrus.Fields{
		"goroutine": gid,
		"name":      name,
		"indent":    indent,
		"parentId":  parentId,
		"params":    paramCount,
		"time":      startTime.Format(TimeFormat),
	}).Info(fmt.Sprintf("%s→ %s", indentStr, name))
}

// logFunctionExit 记录函数退出的日志
func (t *TraceInstance) logFunctionExit(gid uint64, name string, indent int, duration string) {
	// 防止负数导致 panic：indent 是退出前的缩进层级，显示时需要减1
	indentCount := indent - 1
	if indentCount < 0 {
		indentCount = 0
	}
	indentStr := strings.Repeat("  ", indentCount)
	t.log.WithFields(logrus.Fields{
		"goroutine": gid,
		"name":      name,
		"indent":    indentCount,
		"duration":  duration,
	}).Info(fmt.Sprintf("%s← %s (%s)", indentStr, name, duration))
}

// isStructMethod 判断函数名是否为结构体方法，并确定接收者类型
func (t *TraceInstance) isStructMethod(fullName string) FuncInfo {
	// 1. 尝试匹配指针接收者
	if ptrMatches := ptrRegex.FindStringSubmatch(fullName); len(ptrMatches) >= 4 {
		return FuncInfo{
			Type:       MethodTypePointer,
			Package:    ptrMatches[ptrRegex.SubexpIndex("package")],
			StructName: ptrMatches[ptrRegex.SubexpIndex("struct")],
			FuncName:   ptrMatches[ptrRegex.SubexpIndex("method")],
		}
	}

	// 2. 尝试匹配特殊情况（带括号但不是指针接收者）
	if specialMatches := specialValRegex.FindStringSubmatch(fullName); len(specialMatches) >= 4 {
		return FuncInfo{
			Type:       MethodTypeValue, // 这里应该是值接收者方法
			Package:    specialMatches[specialValRegex.SubexpIndex("package")],
			StructName: specialMatches[specialValRegex.SubexpIndex("struct")],
			FuncName:   specialMatches[specialValRegex.SubexpIndex("method")],
		}
	}

	// 3. 尝试匹配值接收者
	if valMatches := valRegex.FindStringSubmatch(fullName); len(valMatches) >= 4 {
		return FuncInfo{
			Type:       MethodTypeValue,
			Package:    valMatches[valRegex.SubexpIndex("package")],
			StructName: valMatches[valRegex.SubexpIndex("struct")],
			FuncName:   valMatches[valRegex.SubexpIndex("method")],
		}
	}

	// 4. 尝试匹配普通函数
	if funcMatches := funcRegex.FindStringSubmatch(fullName); len(funcMatches) >= 3 {
		return FuncInfo{
			Type:     MethodTypeNormal,
			Package:  funcMatches[funcRegex.SubexpIndex("package")],
			FuncName: funcMatches[funcRegex.SubexpIndex("func")],
		}
	}

	// 未知类型
	return FuncInfo{Type: MethodTypeUnknown}
}

func (t *TraceInstance) SkipFunction(name string) bool {
	for _, ignoreName := range t.config.IgnoreNames {
		nameLower := strings.ToLower(name)
		ignoreNameLower := strings.ToLower(ignoreName)
		if strings.Contains(nameLower, ignoreNameLower) {
			return true
		}
	}
	return false
}

// isMainFunction 检查函数名是否为main.main
func (t *TraceInstance) isMainFunction(funcName string) bool {
	// 检查是否为main.main函数
	// 函数名通常是完整路径，如：main.main 或者 github.com/user/project/main.main
	return funcName == "main.main" || strings.HasSuffix(funcName, "/main.main")
}
