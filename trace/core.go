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

	// 获取跟踪信息和全局ID
	indent, parentId, traceId := t.prepareTraceInfo(id)
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

	if t.config.ParamStoreMode != ParamStoreModeNone {
		switch funcInfo.Type {
		case MethodTypeNormal:
			t.DealNormalMethod(traceId, params)
		case MethodTypeValue:
			t.DealValueMethod(traceId, params)
		case MethodTypePointer:
			if t.config.ParamStoreMode == ParamStoreModeNormal {
				// 普通模式下，跳过第一个参数（接收者）
				params = params[1:]
				t.DealNormalMethod(traceId, params)
			} else {
				t.DealPointerMethod(traceId, params)
			}
		}
	}
	traceData.ParamsCount = len(params)
	t.sendOp(&DataOp{
		OpType: OpTypeInsert,
		Arg:    traceData,
	})
	// 记录日志
	t.logFunctionEntry(id, name, indent, parentId, len(params), startTime)
	return traceData, startTime
}

// prepareTraceInfo 准备跟踪信息并返回缩进级别、父ID和新的跟踪ID
func (t *TraceInstance) prepareTraceInfo(id uint64) (indent int, parentId int64, traceId int64) {
	t.Lock()
	defer t.Unlock()

	// 获取或初始化 TraceIndent
	traceIndent, exists := t.indentations[id]
	if !exists {
		traceIndent = &TraceIndent{
			Indent:      0,
			ParentFuncs: make(map[int]int64),
		}
		t.indentations[id] = traceIndent
	}

	// 获取当前缩进和父函数ID
	indent = traceIndent.Indent
	parentId = traceIndent.ParentFuncs[indent-1] // 获取上一层的函数ID作为父函数

	// 生成全局唯一ID
	traceId = t.globalId.Add(1)

	// 更新缩进和父函数ID
	traceIndent.ParentFuncs[indent] = traceId // 使用生成的traceId作为当前函数ID
	traceIndent.Indent++

	return indent, parentId, traceId
}

// ExitTrace 记录函数调用的结束并减少跟踪缩进
func (t *TraceInstance) ExitTrace(info *GoroutineInfo, traceData *model.TraceData, startTime time.Time) {
	// 如果父函数ID为0，并且方法类型为指针接收者，则删除参数缓存
	if traceData.ParentId == 0 && traceData.MethodType == MethodTypePointer {
		t.DeleteParamFromCache(traceData.ID)
	}
	// 更新跟踪信息
	indent := t.updateTraceIndent(info.ID)
	if indent < 0 {
		return // 处理错误情况
	}

	// 计算函数执行时间
	duration := time.Since(startTime)

	t.sendOp(&DataOp{
		OpType: OpTypeUpdate,
		Arg: &model.TraceData{
			ID:         traceData.ID,
			TimeCost:   duration.String(),
			IsFinished: 1,
		},
	})

	// 记录日志
	t.logFunctionExit(info.ID, traceData.Name, indent, duration.String())

	// 检查是否是main.main函数退出，如果是则等待所有数据入库完成
	if t.isMainFunction(traceData.Name) {
		t.log.WithFields(logrus.Fields{
			"function": traceData.Name,
			"indent":   indent,
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
	t.Lock()
	defer t.Unlock()

	// 获取 TraceIndent
	traceIndent, exists := t.indentations[id]
	if !exists {
		t.log.WithFields(logrus.Fields{"goroutine": id}).Error("can't find trace indent")
		return -1
	}

	// 获取当前缩进
	indent := traceIndent.Indent

	// 删除当前层的父函数名称
	delete(traceIndent.ParentFuncs, indent-1)

	// 更新缩进
	traceIndent.Indent--

	// 如果缩进小于等于0，清除所有父函数名称
	if traceIndent.Indent <= 0 {
		traceIndent.ParentFuncs = make(map[int]int64)
	}

	return indent
}

// logFunctionEntry 记录函数进入的日志
func (t *TraceInstance) logFunctionEntry(gid uint64, name string, indent int, parentId int64, paramCount int, startTime time.Time) {
	indentStr := strings.Repeat("  ", indent)
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
	indentStr := strings.Repeat("  ", indent-1)
	t.log.WithFields(logrus.Fields{
		"goroutine": gid,
		"name":      name,
		"indent":    indent - 1,
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
		if strings.Contains(name, ignoreName) {
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
