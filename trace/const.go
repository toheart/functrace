package trace

import (
	"fmt"
	"regexp"
	"time"
)

// 封装 trace 包的常量
const (
	// DefaultMonitorInterval 默认监控间隔（秒）
	DefaultMonitorInterval = 10

	// LogFileName 日志文件名
	LogFileName = "./functrace.log"

	// 内存监控相关常量
	// DefaultMemoryLimit 默认内存限制：2GB
	DefaultMemoryLimit = 2 * 1024 * 1024 * 1024
	// DefaultMemoryCheckInterval 内存监控间隔：5秒
	DefaultMemoryCheckInterval = 5
	// EnvMemoryLimit 内存限制环境变量
	EnvMemoryLimit = "FUNCTRACE_MEMORY_LIMIT"

	// EnvIgnoreNames 忽略的函数名列表环境变量
	EnvIgnoreNames = "FUNCTRACE_IGNORE_NAMES"

	// EnvGoroutineMonitorInterval 协程监控间隔环境变量
	EnvGoroutineMonitorInterval = "FUNCTRACE_GOROUTINE_MONITOR_INTERVAL"

	// EnvMaxDepth 最大深度环境变量
	EnvMaxDepth = "FUNCTRACE_MAX_DEPTH"

	// EnvDBType 数据库类型环境变量
	EnvDBType = "FUNCTRACE_DB_TYPE"
	// 环境变量：数据库插入模式
	// 可选值: "sync"(同步模式，默认), "async"(异步模式)
	EnvDBInsertMode = "ENV_DB_INSERT_MODE"

	// EnvParamStoreMode 参数存储模式环境变量
	// 可选值: "none"(不保存参数，默认), "normal"(保存普通参数), "all"(全保存)
	EnvParamStoreMode = "FUNCTRACE_PARAM_STORE_MODE"

	IgnoreNames = "context,string"
	// 默认最大深度
	DefaultMaxDepth = 3
	// 时间格式
	TimeFormat = time.RFC3339Nano
)

// 数据库插入模式
const (
	// 异步插入模式
	AsyncMode = "async"
	// 同步插入模式
	SyncMode = "sync"
)

// 参数存储模式
const (
	// 不保存参数（默认模式，内存友好）
	ParamStoreModeNone = "none"
	// 保存普通参数（不包括指针接收者的复杂参数处理）
	ParamStoreModeNormal = "normal"
	// 全保存（包括所有参数和指针接收者的diff处理）
	ParamStoreModeAll = "all"
)

// 方法类型常量
const (
	MethodTypeUnknown = iota
	MethodTypeNormal
	MethodTypeValue
	MethodTypePointer
)

// OpType 操作类型
type OpType int

const (
	OpTypeInsert OpType = iota
	OpTypeUpdate
)

var (
	// 指针接收者函数：pkg.(*Type).Method
	ptrRegex = regexp.MustCompile(`^(?P<package>.*)\.\(\*(?P<struct>\w+)\)\.(?P<method>\w+)$`)

	// 值接收者函数：pkg.Type.Method
	valRegex = regexp.MustCompile(`^(?P<package>.*)\.(?P<struct>\w+)\.(?P<method>\w+)$`)

	// 特殊情况：带括号但不是指针接收者的方法，如 pkg.(Type).Method
	specialValRegex = regexp.MustCompile(`^(?P<package>.*)\.\((?P<struct>\w+)\)\.(?P<method>\w+)$`)

	// 普通函数：pkg.Func
	funcRegex = regexp.MustCompile(`^(?P<package>.*)\.(?P<func>\w+)$`)
)

type FuncInfo struct {
	Type       int    // 类型：pointer_receiver/value_receiver/plain_func
	Package    string // 包路径
	StructName string // 结构体名（若有）
	FuncName   string // 函数名
}

func (f FuncInfo) String() string {
	return fmt.Sprintf("%s.(*%s)", f.Package, f.StructName)
}
