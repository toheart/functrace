package trace

import (
	"runtime"
	"sync"
)

// 独立组件：基于 PC 的函数名/跳过判定缓存
var (
	pcCacheMu   sync.RWMutex
	pcNameCache = make(map[uintptr]string)
	pcSkipCache = make(map[uintptr]bool)
)

// ShouldSkipPC 基于 PC 进行快速跳过判断，并返回是否跳过与函数名
// decide 回调用于根据函数名判定是否跳过（例如调用 TraceInstance.SkipFunction）
func ShouldSkipPC(pc uintptr, decide func(string) bool) (bool, string) {
	// 先读跳过缓存
	pcCacheMu.RLock()
	if skip, ok := pcSkipCache[pc]; ok {
		name := pcNameCache[pc]
		pcCacheMu.RUnlock()
		return skip, name
	}
	name := pcNameCache[pc]
	pcCacheMu.RUnlock()

	// 未命名则解析函数名
	if name == "" {
		if fn := runtime.FuncForPC(pc); fn != nil {
			name = fn.Name()
		}
	}

	// 由外部策略决定是否跳过
	skip := decide(name)

	// 回写缓存
	pcCacheMu.Lock()
	pcNameCache[pc] = name
	pcSkipCache[pc] = skip
	pcCacheMu.Unlock()

	return skip, name
}
