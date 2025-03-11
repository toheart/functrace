package functrace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Trace 是一个装饰器，用于跟踪函数的进入和退出
func Trace(params []interface{}) func() {
	// 获取 TraceInstance 单例
	instance := NewTraceInstance()

	// 获取调用者信息
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		instance.log.Error("can't find caller")
		return func() {}
	}

	// 获取 goroutine ID 和函数名
	id := getGID()
	fn := runtime.FuncForPC(pc)
	name := fn.Name()

	// 检查是否应该跳过此函数
	if skipFunction(name) {
		return func() {}
	}

	// 确保 TraceIndent 已初始化
	instance.initTraceIndentIfNeeded(id)

	// 记录函数进入
	traceId, startTime := instance.enterTrace(id, name, params)

	// 返回用于记录函数退出的闭包
	return func() {
		instance.exitTrace(id, name, startTime, traceId)
	}
}

// enterTrace 记录函数调用的开始并存储必要的跟踪详情
func (t *TraceInstance) enterTrace(id uint64, name string, params []interface{}) (traceId int64, startTime time.Time) {
	startTime = time.Now() // 记录开始时间

	// 获取跟踪信息和全局ID
	indent, parentId, traceId := t.prepareTraceInfo(id, name)

	// 准备参数输出
	traceParams := prepareParamsOutput(params)
	paramsJSON, err := json.Marshal(traceParams)
	if err != nil {
		t.log.Error("can't convert params to json", "error", err)
		return traceId, startTime
	}

	// 创建跟踪数据
	traceData := TraceData{
		ID:        traceId,
		Name:      name,
		GID:       id,
		Indent:    indent,
		ParentId:  parentId,
		CreatedAt: time.Now().Format(TimeFormat),
		Seq:       time.Since(currentNow).String(),
	}

	// 发送数据库插入操作
	t.sendDBOperation(id, OpTypeInsert, SQLInsertTrace, []interface{}{
		traceData.ID, traceData.Name, traceData.GID, traceData.Indent,
		paramsJSON, traceData.ParentId, traceData.CreatedAt, traceData.Seq,
	})

	// 记录日志
	t.logFunctionEntry(id, name, indent, parentId, string(paramsJSON), startTime)

	return traceId, startTime
}

// prepareTraceInfo 准备跟踪信息并返回缩进级别、父ID和新的跟踪ID
func (t *TraceInstance) prepareTraceInfo(id uint64, name string) (indent int, parentId int64, traceId int64) {
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

// exitTrace 记录函数调用的结束并减少跟踪缩进
func (t *TraceInstance) exitTrace(id uint64, name string, startTime time.Time, traceId int64) {
	// 更新跟踪信息
	indent := t.updateTraceIndent(id)
	if indent < 0 {
		return // 处理错误情况
	}

	// 计算函数执行时间
	duration := time.Since(startTime)
	durationStr := formatDuration(duration)

	// 发送数据库更新操作
	t.sendDBOperation(id, OpTypeUpdate, SQLUpdateTimeCost, []interface{}{
		duration.String(), traceId,
	})

	// 记录日志
	t.logFunctionExit(id, name, indent, durationStr)
}

// updateTraceIndent 更新跟踪缩进并返回当前缩进级别
func (t *TraceInstance) updateTraceIndent(id uint64) int {
	t.Lock()
	defer t.Unlock()

	// 获取 TraceIndent
	traceIndent, exists := t.indentations[id]
	if !exists {
		t.log.Error("can't find TraceIndent for goroutine", "goroutine", id)
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

// getGID 获取当前goroutine的ID
func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

// skipFunction 检查是否应该跳过跟踪某个函数
func skipFunction(name string) bool {
	ignoreEnv := os.Getenv(EnvIgnoreNames)
	var ignoreNames []string
	if ignoreEnv != "" {
		ignoreNames = strings.Split(ignoreEnv, ",")
	} else {
		ignoreNames = strings.Split(IgnoreNames, ",")
	}

	for _, ignoreName := range ignoreNames {
		if strings.Contains(strings.ToLower(name), ignoreName) {
			return true
		}
	}
	return false
}

// formatDuration 格式化持续时间，使其更易读
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%d ns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.2f µs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.2f ms", float64(d.Nanoseconds())/1000000)
	} else {
		return fmt.Sprintf("%.2f s", d.Seconds())
	}
}
