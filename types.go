package functrace

// OpType 定义数据库操作类型
type OpType int

// 数据库操作类型常量
const (
	OpTypeInsert OpType = iota // 插入操作
	OpTypeUpdate               // 更新操作
)

// TraceData 存储跟踪数据的结构体
type TraceData struct {
	ID        int64  `json:"id"`        // 唯一标识符
	Name      string `json:"name"`      // 函数名称
	GID       uint64 `json:"gid"`       // Goroutine ID
	Indent    int    `json:"indent"`    // 缩进级别
	Params    string `json:"params"`    // 参数JSON字符串
	TimeCost  string `json:"timeCost"`  // 执行时间
	ParentId  int64  `json:"parentId"`  // 父函数ID
	CreatedAt string `json:"createdAt"` // 创建时间
	Seq       string `json:"seq"`       // 序列号
}

// GoroutineTrace 存储goroutine信息的结构体
type GoroutineTrace struct {
	ID           int64  `json:"id"`           // 自增ID
	GID          uint64 `json:"gid"`          // Goroutine ID
	TimeCost     string `json:"timeCost"`     // 执行时间
	CreateTime   string `json:"createTime"`   // 创建时间
	IsFinished   int    `json:"isFinished"`   // 是否完成
	InitFuncName string `json:"initFuncName"` // 初始函数名
}

// TraceParams 存储参数信息的结构体
type TraceParams struct {
	Pos   int    // 参数位置
	Param string // 参数格式化后的字符串
}

// TraceIndent 存储函数调用的缩进信息和父函数名称
type TraceIndent struct {
	Indent      int           // 当前缩进级别
	ParentFuncs map[int]int64 // 每一层当前父函数ID
}

// dbOperation 定义数据库操作
type dbOperation struct {
	opType OpType        // 操作类型
	query  string        // SQL查询语句
	args   []interface{} // 查询参数
}

type GoroutineInfo struct {
	ID             uint64 `json:"id"`             // 自增ID
	Gid            uint64 `json:"gid"`            // Goroutine ID
	LastUpdateTime string `json:"lastUpdateTime"` // 最后更新时间
}
