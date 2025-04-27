package sqlite

// 数据库相关常量
const (
	// 数据库文件名格式
	DBFileNameFormat = "./%s_%s.db"

	// SQL语句
	SQLCreateTraceTable = `CREATE TABLE IF NOT EXISTS TraceData (
		id INTEGER PRIMARY KEY, 
		name TEXT, 
		gid INTEGER, 
		indent INTEGER, 
		paramsCount INTEGER, 
		timeCost TEXT, 
		parentId INTEGER, 
		isFinished INTEGER,
		createdAt TEXT, 
		seq TEXT
	)`
	// Goroutine表创建语句
	SQLCreateGoroutineTable = `CREATE TABLE IF NOT EXISTS GoroutineTrace (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		originGid INTEGER, 
		timeCost TEXT, 
		createTime TEXT, 
		isFinished INTEGER, 
		initFuncName TEXT
	)`

	// 参数表创建语句
	SQLCreateParamTable = `CREATE TABLE IF NOT EXISTS ParamStore (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		traceId INTEGER, 
		position INTEGER, 
		data TEXT, 
		isReceiver BOOLEAN, 
		baseId INTEGER
	)`

	SQLCreateGIDIndex        = "CREATE INDEX IF NOT EXISTS idx_gid ON TraceData (gid)"
	SQLCreateParentIndex     = "CREATE INDEX IF NOT EXISTS idx_parent ON TraceData (parentId)"
	SQLCreateParamTraceIndex = "CREATE INDEX IF NOT EXISTS idx_param_trace ON ParamStore (traceId)"
	SQLCreateParamBaseIndex  = "CREATE INDEX IF NOT EXISTS idx_param_base ON ParamStore (baseId)"

	SQLInsertTrace    = "INSERT INTO TraceData (id, name, gid, indent, paramsCount, parentId, createdAt, seq) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	SQLUpdateTimeCost = "UPDATE TraceData SET timeCost = ?, isFinished = ? WHERE id = ?"

	// 参数表操作语句
	SQLInsertParam = "INSERT INTO ParamStore (traceId, position, data, isReceiver, baseId) VALUES (?, ?, ?, ?, ?)"

	// Goroutine表操作语句
	SQLInsertGoroutine         = "INSERT INTO GoroutineTrace (id, originGid, createTime, isFinished, initFuncName) VALUES (?, ?, ?, ?, ?)"
	SQLUpdateGoroutineTimeCost = "UPDATE GoroutineTrace SET timeCost = ?, isFinished = ? WHERE id = ?"

	// 查询特定goroutine的根函数调用
	SQLQueryRootFunctions = "SELECT id, timeCost FROM TraceData WHERE gid = ? AND indent = 0"
)
