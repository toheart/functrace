package functrace

import "time"

// 常量定义
const (
	// IgnoreNames 定义默认忽略的函数名称关键字
	IgnoreNames = "log,context"

	// 环境变量名称
	EnvTraceChannelCount        = "TRACE_CHANNEL_COUNT"
	EnvIgnoreNames              = "IGNORE_NAMES"
	EnvGoroutineMonitorInterval = "GOROUTINE_MONITOR_INTERVAL" // 监控goroutine的时间间隔(秒)

	// 默认配置值
	DefaultChannelCount      = 10 // 通道数量
	DefaultChannelBufferSize = 20 // 通道缓冲区大小
	DefaultMonitorInterval   = 60 // 默认监控间隔，单位秒

	// 日志文件配置
	LogFileName = "./trace.log"

	// 数据库相关
	DBFileNameFormat = "./trace_%s_%s.db"

	// SQL语句
	SQLCreateTable = `CREATE TABLE IF NOT EXISTS TraceData (
		id INTEGER PRIMARY KEY, 
		name TEXT, 
		gid INTEGER, 
		indent INTEGER, 
		params TEXT, 
		timeCost TEXT, 
		parentId INTEGER, 
		createdAt TEXT, 
		seq TEXT
	)`
	// Goroutine表创建语句
	SQLCreateGoroutineTable = `CREATE TABLE IF NOT EXISTS GoroutineTrace (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		gid INTEGER, 
		timeCost TEXT, 
		createTime TEXT, 
		isFinished INTEGER, 
		initFuncName TEXT
	)`

	SQLCreateGIDIndex          = "CREATE INDEX IF NOT EXISTS idx_gid ON TraceData (gid)"
	SQLCreateParentIndex       = "CREATE INDEX IF NOT EXISTS idx_parent ON TraceData (parentId)"
	SQLCreateGoroutineGIDIndex = "CREATE INDEX IF NOT EXISTS idx_goroutine_gid ON GoroutineTrace (gid)"
	SQLInsertTrace             = "INSERT INTO TraceData (id, name, gid, indent, params, parentId, createdAt, seq) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	SQLUpdateTimeCost          = "UPDATE TraceData SET timeCost = ? WHERE id = ?"

	// Goroutine表操作语句
	SQLInsertGoroutine         = "INSERT INTO GoroutineTrace (id, gid, createTime, isFinished, initFuncName) VALUES (?, ?, ?, ?, ?)"
	SQLUpdateGoroutineTimeCost = "UPDATE GoroutineTrace SET timeCost = ?, isFinished = ? WHERE id = ?"

	// 查询特定goroutine的根函数调用
	SQLQueryRootFunctions = "SELECT id, timeCost FROM TraceData WHERE gid = ? AND indent = 0"

	// 缩进格式
	IndentFormat = "**"

	// 时间格式
	TimeFormat           = time.RFC3339Nano
	TimeFormatWithMillis = "15:04:05.000"
)
