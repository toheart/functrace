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

	"github.com/davecgh/go-spew/spew"
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
	gid := getGID()
	fn := runtime.FuncForPC(pc)
	name := fn.Name()
	// 检查是否应该跳过此函数
	if skipFunction(name) {
		return func() {}
	}

	// 修改id值
	info, _ := instance.initGoroutineIfNeeded(gid, name)

	// 确保 TraceIndent 已初始化
	instance.initTraceIndentIfNeeded(info.ID)

	// 记录函数进入
	traceId, startTime := instance.enterTrace(info.ID, name, params)

	// 返回用于记录函数退出的闭包
	return func() {
		instance.exitTrace(info, name, startTime, traceId)
	}
}

// enterTrace 记录函数调用的开始并存储必要的跟踪详情
func (t *TraceInstance) enterTrace(id uint64, name string, params []interface{}) (traceId int64, startTime time.Time) {
	startTime = time.Now() // 记录开始时间

	// 获取跟踪信息和全局ID
	indent, parentId, traceId := t.prepareTraceInfo(id)

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
		CreatedAt: startTime.Format(TimeFormat),
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

// exitTrace 记录函数调用的结束并减少跟踪缩进
func (t *TraceInstance) exitTrace(info *GoroutineInfo, name string, startTime time.Time, traceId int64) {
	// 更新跟踪信息
	indent := t.updateTraceIndent(info.ID)
	if indent < 0 {
		return // 处理错误情况
	}

	// 计算函数执行时间
	duration := time.Since(startTime)
	durationStr := formatDuration(duration)

	// 发送数据库更新操作
	t.sendDBOperation(info.ID, OpTypeUpdate, SQLUpdateTimeCost, []interface{}{
		duration.String(), traceId,
	})

	// 记录日志
	t.logFunctionExit(info.ID, name, indent, durationStr)
	// 更新goroutine的最后更新时间
	info.LastUpdateTime = time.Now().Format(TimeFormat)
	go t.SetGoroutineRunning(info)
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
	name = strings.ToLower(name)
	_, ok := singleTrace.IgnoreNames[name]
	return ok
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

// initGoroutineIfNeeded 检查goroutine是否已在跟踪中，如果不在则初始化
func (t *TraceInstance) initGoroutineIfNeeded(gid uint64, name string) (info *GoroutineInfo, initFunc bool) {
	// 判断该goroutine是否已经被跟踪
	t.RLock()
	info, exists := t.GoroutineRunning[gid]
	t.RUnlock()

	if exists {
		return info, false
	}

	t.Lock()
	defer t.Unlock()
	// 二次检查
	info, exists = t.GoroutineRunning[gid]
	if exists {
		return info, false
	}
	// 更新运行中的goroutine映射
	start := time.Now()
	id := t.gGroutineId.Add(1)
	info = &GoroutineInfo{
		ID:             id,
		OriginGID:      gid,
		LastUpdateTime: start.Format(TimeFormat),
	}
	t.GoroutineRunning[gid] = info
	// 异步插入goroutine数据
	go func(id uint64, gid uint64, funcName string, createTimeStr string) {
		// 执行插入操作
		_, err := t.db.Exec(SQLInsertGoroutine, id, gid, createTimeStr, 0, funcName)
		if err != nil {
			t.log.Error("failed to insert goroutine trace", "error", err)
			return
		}

		t.log.Info("initialized goroutine trace", "goroutine", id, "initFunc", funcName)
	}(id, gid, name, start.Format(TimeFormat))

	return info, true
}

func (t *TraceInstance) SetGoroutineRunning(info *GoroutineInfo) {
	t.Lock()
	t.GoroutineRunning[info.OriginGID] = info
	t.Unlock()
}

func (t *TraceInstance) GoroutineFinished(info *GoroutineInfo) {
	t.Lock()
	delete(t.GoroutineRunning, info.OriginGID)
	t.Unlock()
}

// finishGoroutineTrace 完成对goroutine的跟踪
func (t *TraceInstance) finishGoroutineTrace(info *GoroutineInfo) {
	t.log.Info("finishing goroutine trace", "id", info.ID)
	// 获取goroutine创建时间
	createTimeStr, err := t.getGoroutineCreateTime(info.ID)
	if err != nil {
		t.log.Error("failed to get goroutine create time", "error", err)
		// 在无法获取创建时间的情况下，使用根函数执行时间作为总时间
		t.updateGoroutineTimeCost(info.ID, time.Since(currentNow).String(), 1)
		return
	}

	// 解析创建时间
	createTime, err := time.Parse(TimeFormat, createTimeStr)
	if err != nil {
		t.log.Error("failed to parse create time", "error", err)
		// 在解析失败的情况下，使用根函数执行时间作为总时间
		t.updateGoroutineTimeCost(info.ID, time.Since(currentNow).String(), 1)
		return
	}
	// 获取goroutine最后更新时间
	lastTime, err := time.Parse(TimeFormat, info.LastUpdateTime)
	if err != nil {
		t.log.Error("failed to parse last update time", "error", err)
		return
	}
	// 计算总运行时间（当前时间 - 创建时间）
	totalExecTime := lastTime.Sub(createTime)

	// 更新goroutine数据，timeCost为总运行时间
	t.updateGoroutineTimeCost(info.ID, totalExecTime.String(), 1)

	// 从映射中移除
	t.deleteGoroutineRunning(info.OriginGID)
	t.log.Info("completed goroutine trace",
		"goroutine identifier", info.OriginGID,
		"database id", info.ID,
		"total execution time", totalExecTime.String())
}

// getGoroutineCreateTime 获取goroutine创建时间
func (t *TraceInstance) getGoroutineCreateTime(id uint64) (string, error) {
	var createTimeStr string
	err := t.db.QueryRow("SELECT createTime FROM GoroutineTrace WHERE id = ?", id).Scan(&createTimeStr)
	return createTimeStr, err
}

// updateGoroutineTimeCost 更新goroutine时间成本
func (t *TraceInstance) updateGoroutineTimeCost(id uint64, timeCostStr string, isFinished int) {
	t.log.Info("updating goroutine trace with time cost", "id", id, "timeCost", timeCostStr, "isFinished", isFinished)
	_, err := t.db.Exec(SQLUpdateGoroutineTimeCost, timeCostStr, isFinished, id)
	if err != nil {
		t.log.Error("failed to update goroutine trace with time cost", "error", err)
	}
}

// deleteGoroutineRunning 从映射中移除goroutine
func (t *TraceInstance) deleteGoroutineRunning(gid uint64) {
	t.Lock()
	delete(t.GoroutineRunning, gid)
	t.Unlock()
}

// monitorGoroutines 定时监控goroutine的运行状态
func (t *TraceInstance) monitorGoroutines() {
	// 获取监控间隔时间
	interval := DefaultMonitorInterval
	if intervalStr := os.Getenv(EnvGoroutineMonitorInterval); intervalStr != "" {
		if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
			interval = i
		}
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.checkAndFinishGoroutines()
		case <-stopMonitor:
			t.log.Info("goroutine monitor stopped")
			return
		}
	}
}

func (t *TraceInstance) GetGoroutineRunning() map[uint64]*GoroutineInfo {
	t.RLock()
	defer t.RUnlock()
	runningGoroutines := make(map[uint64]*GoroutineInfo)
	for k, v := range t.GoroutineRunning {
		runningGoroutines[k] = v
	}
	return runningGoroutines
}

// checkAndFinishGoroutines 检查并完成已结束的goroutine跟踪
func (t *TraceInstance) checkAndFinishGoroutines() {
	t.log.Info("checking and finishing finished goroutine traces")
	// 获取当前运行的所有goroutine ID
	currentGoroutines := make(map[uint64]bool)
	for _, id := range t.getAllGoroutineIDs() {
		currentGoroutines[uint64(id)] = true
	}

	// 复制当前运行中的goroutine映射，以避免在迭代过程中修改
	var finishedGoroutines []*GoroutineInfo

	runningGoroutines := t.GetGoroutineRunning()

	// 检查哪些goroutine已经结束但未更新
	for _, info := range runningGoroutines {
		if _, exists := currentGoroutines[info.OriginGID]; !exists {
			// 该goroutine不再运行，需要完成其跟踪
			finishedGoroutines = append(finishedGoroutines, info)
		}
	}
	t.log.Info("finished goroutines", "count", len(finishedGoroutines))
	// 处理已结束的goroutine
	for _, info := range finishedGoroutines {
		t.finishGoroutineTrace(info)
	}

	t.log.Info("goroutine monitor check completed",
		"running count", len(runningGoroutines),
		"finished count", len(finishedGoroutines))
}

// prepareParamsOutput 准备参数输出
func prepareParamsOutput(params []interface{}) []*TraceParams {
	var traceParams []*TraceParams

	// 如果没有参数，返回一个特殊标记
	if len(params) == 0 {
		return nil
	}

	// 处理参数
	for i, item := range params {
		traceParams = append(traceParams, &TraceParams{
			Pos: i,
			// Param: formatParam(i, item),
			Param: spew.Sdump(item),
		})
	}

	return traceParams
}
