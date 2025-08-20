// Package functrace 提供了用于跟踪函数执行的工具
// 这个包是 trace 子包的外层接口，提供简单的 API 供用户使用
package functrace

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/trace"
)

// Trace 是一个装饰器，用于跟踪函数的进入和退出
func Trace(params []interface{}) func() {
	// 获取 TraceInstance 单例
	instance := trace.NewTraceInstance()

	// 获取调用者信息（PC）
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		instance.GetLogger().WithFields(nil).Error("can't get caller info")
		return func() {}
	}

	// 基于 PC 的快速跳过判断
	skip, name := trace.ShouldSkipPC(pc, instance.SkipFunction)
	if skip {
		instance.GetLogger().WithFields(logrus.Fields{"name": name}).Info("skip function")
		return func() {}
	}
	if name == "" {
		if fn := runtime.FuncForPC(pc); fn != nil {
			name = fn.Name()
		}
	}

	// 仅在未跳过时获取 goroutine ID（缓冲池）
	gid := getGID()

	instance.GetLogger().WithFields(logrus.Fields{"name": name}).Info("enter function")
	// 原子化地初始化goroutine和trace缩进，避免并发安全问题
	info, _ := instance.InitGoroutineAndTraceAtomic(gid, name)

	// 记录函数进入
	traceData, startTime := instance.EnterTrace(info.ID, name, params)

	// 返回用于记录函数退出的闭包
	return func() {
		instance.ExitTrace(info, traceData, startTime)
	}
}

// Close 关闭跟踪实例并释放资源
func CloseTraceInstance() error {
	return trace.GetTraceInstance().Close()
}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	return trace.GetTraceInstance().GetLogger()
}

// getGID 获取当前goroutine的ID
var gidBufPool = sync.Pool{New: func() interface{} { return make([]byte, 64) }}

func getGID() uint64 {
	b := gidBufPool.Get().([]byte)
	n := runtime.Stack(b, false)
	if n > len(b) {
		n = len(b)
	}
	s := b[:n]
	s = bytes.TrimPrefix(s, []byte("goroutine "))
	if i := bytes.IndexByte(s, ' '); i >= 0 {
		s = s[:i]
	}
	id, _ := strconv.ParseUint(string(s), 10, 64)
	gidBufPool.Put(b)
	return id
}
