package objectdump

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// LargeObjectHandler 定义了大对象处理器的接口
type LargeObjectHandler func(v reflect.Value) (string, bool)

// largeObjectRegistry 存储大对象类型的处理器
var largeObjectRegistry = make(map[string]LargeObjectHandler)

// 初始化默认的大对象处理器
func init() {
	// 时间相关类型
	RegisterLargeObject("time.Time", func(v reflect.Value) (string, bool) {
		if v.CanInterface() {
			if t, ok := v.Interface().(time.Time); ok {
				return t.String(), true
			}
		}
		return v.String(), true
	})

	RegisterLargeObject("time.Duration", func(v reflect.Value) (string, bool) {
		if v.CanInterface() {
			if d, ok := v.Interface().(time.Duration); ok {
				return d.String(), true
			}
		}
		return v.String(), true
	})

	RegisterLargeObject("time.Location", func(v reflect.Value) (string, bool) {
		if v.CanInterface() {
			if loc, ok := v.Interface().(*time.Location); ok {
				return loc.String(), true
			}
		}
		return "<time.Location>", true
	})

	// 网络相关类型
	RegisterLargeObject("net.Conn", func(v reflect.Value) (string, bool) {
		return "<net.Conn>", true
	})

	RegisterLargeObject("http.Request", func(v reflect.Value) (string, bool) {
		return "<http.Request>", true
	})

	RegisterLargeObject("http.Response", func(v reflect.Value) (string, bool) {
		return "<http.Response>", true
	})

	// 文件系统相关类型
	RegisterLargeObject("os.File", func(v reflect.Value) (string, bool) {
		return "<os.File>", true
	})

	// 数据库相关类型
	RegisterLargeObject("sql.DB", func(v reflect.Value) (string, bool) {
		return "<sql.DB>", true
	})

	RegisterLargeObject("sql.Rows", func(v reflect.Value) (string, bool) {
		return "<sql.Rows>", true
	})

	RegisterLargeObject("sql.Stmt", func(v reflect.Value) (string, bool) {
		return "<sql.Stmt>", true
	})

	// 反射相关类型
	RegisterLargeObject("reflect.Value", func(v reflect.Value) (string, bool) {
		return "<reflect.Value>", true
	})

	// sync包相关类型
	RegisterLargeObject("sync.Mutex", func(v reflect.Value) (string, bool) { return "<sync.Mutex>", true })
	RegisterLargeObject("sync.RWMutex", func(v reflect.Value) (string, bool) { return "<sync.RWMutex>", true })
	RegisterLargeObject("sync.Cond", func(v reflect.Value) (string, bool) { return "<sync.Cond>", true })
	RegisterLargeObject("sync.Pool", func(v reflect.Value) (string, bool) { return "<sync.Pool>", true })
	RegisterLargeObject("sync.WaitGroup", func(v reflect.Value) (string, bool) { return "<sync.WaitGroup>", true })
	RegisterLargeObject("sync.Once", func(v reflect.Value) (string, bool) { return "<sync.Once>", true })

	// context包相关类型
	RegisterLargeObject("context.backgroundCtx", func(v reflect.Value) (string, bool) { return "context.Background", true })
	RegisterLargeObject("context.emptyCtx", func(v reflect.Value) (string, bool) { return "context.TODO", true })
	RegisterLargeObject("context.cancelCtx", func(v reflect.Value) (string, bool) { return "<context.CancelCtx>", true })
	RegisterLargeObject("context.timerCtx", func(v reflect.Value) (string, bool) { return "<context.TimerCtx>", true })
	RegisterLargeObject("context.valueCtx", func(v reflect.Value) (string, bool) { return "<context.ValueCtx>", true })

	// 通用处理器：处理特定包下的类型
	RegisterLargeObject("time.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})

	RegisterLargeObject("net.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})

	RegisterLargeObject("http.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})

	RegisterLargeObject("os.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})

	RegisterLargeObject("sql.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})

	RegisterLargeObject("crypto.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})

	RegisterLargeObject("runtime.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})

	RegisterLargeObject("reflect.", func(v reflect.Value) (string, bool) {
		return fmt.Sprintf("<%s>", v.Type().String()), true
	})
}

// RegisterLargeObject 注册一个大对象处理器
func RegisterLargeObject(typePattern string, handler LargeObjectHandler) {
	largeObjectRegistry[typePattern] = handler
}

// UnregisterLargeObject 注销一个大对象处理器
func UnregisterLargeObject(typePattern string) {
	delete(largeObjectRegistry, typePattern)
}

// ClearLargeObjectRegistry 清空所有大对象处理器
func ClearLargeObjectRegistry() {
	largeObjectRegistry = make(map[string]LargeObjectHandler)
}

// GetLargeObjectRegistry 获取当前注册的所有大对象处理器
func GetLargeObjectRegistry() map[string]LargeObjectHandler {
	result := make(map[string]LargeObjectHandler)
	for k, v := range largeObjectRegistry {
		result[k] = v
	}
	return result
}

// handleLargeObject 处理大对象，返回处理结果和是否被处理
func handleLargeObject(v reflect.Value) (string, bool) {
	typeName := v.Type().String()

	// 首先尝试精确匹配
	if handler, exists := largeObjectRegistry[typeName]; exists {
		return handler(v)
	}

	// 然后尝试包前缀匹配
	for pattern, handler := range largeObjectRegistry {
		if strings.HasSuffix(pattern, ".") && strings.HasPrefix(typeName, pattern) {
			return handler(v)
		}
	}

	return "", false
}
