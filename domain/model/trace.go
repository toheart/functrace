package model

// TraceData 存储跟踪数据的结构体
type TraceData struct {
	ID          int64  `json:"id"`          // 唯一标识符
	Name        string `json:"name"`        // 函数名称
	GID         uint64 `json:"gid"`         // Goroutine ID
	Indent      int    `json:"indent"`      // 缩进级别
	ParamsCount int    `json:"paramsCount"` // 参数数量
	TimeCost    string `json:"timeCost"`    // 执行时间
	ParentId    int64  `json:"parentId"`    // 父函数ID
	CreatedAt   string `json:"createdAt"`   // 创建时间
	IsFinished  int    `json:"isFinished"`  // 是否完成
	Seq         string `json:"seq"`         // 序列号
	MethodType  int    `json:"-"`           // 方法类型
}

// GoroutineTrace 存储goroutine信息的结构体
type GoroutineTrace struct {
	ID           int64  `json:"id"`           // 自增ID
	OriginGID    uint64 `json:"originGid"`    // 原始Goroutine ID
	TimeCost     string `json:"timeCost"`     // 执行时间
	CreateTime   string `json:"createTime"`   // 创建时间
	IsFinished   int    `json:"isFinished"`   // 是否完成
	InitFuncName string `json:"initFuncName"` // 初始函数名
}

// TraceIndent 存储函数调用的缩进信息和父函数名称
type TraceIndent struct {
	Indent      int           // 当前缩进级别
	ParentFuncs map[int]int64 // 每一层当前父函数ID
}

// GoroutineInfo 协程状态信息
type GoroutineInfo struct {
	ID             uint64 `json:"id"`             // 自增ID
	OriginGID      uint64 `json:"originGid"`      // 原始Goroutine ID
	LastUpdateTime string `json:"lastUpdateTime"` // 最后更新时间
}

// New TraceData 创建一个新的跟踪数据
func NewTraceData(id int64, name string, gid uint64, indent int, paramsCount int, parentId int64, createdAt string, seq string) *TraceData {
	return &TraceData{
		ID:          id,
		Name:        name,
		GID:         gid,
		Indent:      indent,
		ParamsCount: paramsCount,
		ParentId:    parentId,
		CreatedAt:   createdAt,
		Seq:         seq,
	}
}

// WithTimeCost 设置执行时间
func (t *TraceData) WithTimeCost(timeCost string) *TraceData {
	t.TimeCost = timeCost
	return t
}

// NewGoroutineTrace 创建一个新的goroutine跟踪数据
func NewGoroutineTrace(id int64, originGid uint64, createTime string, isFinished int, initFuncName string) *GoroutineTrace {
	return &GoroutineTrace{
		ID:           id,
		OriginGID:    originGid,
		CreateTime:   createTime,
		IsFinished:   isFinished,
		InitFuncName: initFuncName,
	}
}

// WithTimeCost 设置执行时间
func (g *GoroutineTrace) WithTimeCost(timeCost string) *GoroutineTrace {
	g.TimeCost = timeCost
	return g
}

// SetFinished 设置是否完成
func (g *GoroutineTrace) SetFinished(isFinished int) *GoroutineTrace {
	g.IsFinished = isFinished
	return g
}
