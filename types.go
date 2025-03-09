package functrace

type TraceData struct {
	ID             int64         `json:"id"`
	Name           string        `json:"name"`
	GID            uint64        `json:"gid"`
	Indent         int           `json:"indent"`
	Params         []TraceParams `json:"params"`
	TimeCost       string        `json:"timeCost"`
	ParentFuncName string        `json:"parentFuncname"`
	CreatedAt      string        `json:"createdAt"`
	Seq            string        `json:"seq"`
}

type TraceParams struct {
	Pos   int    // 记录参数的位置
	Param string // 记录函数参数
}

// TraceIndent 存储函数调用的缩进信息和父函数名称
type TraceIndent struct {
	Indent      int            // 当前缩进级别
	ParentFuncs map[int]string // 每一层的父函数名称
}

// dbOperation 定义数据库操作
type dbOperation struct {
	query string
	args  []interface{}
}
