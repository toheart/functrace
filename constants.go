package functrace

// 常量定义
const (
	// IgnoreNames 定义默认忽略的函数名称关键字
	IgnoreNames = "log,context"

	// 环境变量名称
	EnvTraceChannelCount = "TRACE_CHANNEL_COUNT"
	EnvIgnoreNames       = "IGNORE_NAMES"

	// 默认配置值
	DefaultChannelCount      = 5
	DefaultChannelBufferSize = 100
	DefaultPoolSize          = 10

	// 日志文件配置
	LogFileName = "./trace.log"

	// 数据库相关
	DBFileNameFormat = "%s_trace_%d.db"

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
	SQLCreateGIDIndex    = "CREATE INDEX IF NOT EXISTS idx_gid ON TraceData (gid)"
	SQLCreateParentIndex = "CREATE INDEX IF NOT EXISTS idx_parent ON TraceData (parentId)"
	SQLInsertTrace       = "INSERT INTO TraceData (id, name, gid, indent, params, parentId, createdAt, seq) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	SQLUpdateTimeCost    = "UPDATE TraceData SET timeCost = ? WHERE id = ?"

	// 缩进格式
	IndentFormat = "**"

	// 时间格式
	TimeFormat           = "2006-01-02 15:04:05"
	TimeFormatWithMillis = "15:04:05.000"
)
