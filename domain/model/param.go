package model

// ParamStoreData 存储参数信息的结构体
type ParamStoreData struct {
	ID         int64  `json:"id"`         // 唯一标识符
	TraceID    int64  `json:"traceId"`    // 关联的TraceData ID
	Position   int    `json:"position"`   // 参数位置
	Data       string `json:"data"`       // 参数JSON数据
	IsReceiver bool   `json:"isReceiver"` // 是否为接收者参数
	BaseID     int64  `json:"baseId"`     // 基础参数ID（自关联，当参数为增量存储时使用）
}

// ReceiverInfo 接收者信息
type ReceiverInfo struct {
	BaseID int64  // 基础参数ID
	Data   string // JSON格式的参数数据
}

// NewParamStoreData 创建一个新的参数存储数据
func NewParamStoreData(traceId int64, position int, data string, isReceiver bool, baseId int64) *ParamStoreData {
	return &ParamStoreData{
		TraceID:    traceId,
		Position:   position,
		Data:       data,
		IsReceiver: isReceiver,
		BaseID:     baseId,
	}
}

// WithID 设置ID
func (p *ParamStoreData) WithID(id int64) *ParamStoreData {
	p.ID = id
	return p
}

// NewReceiverInfo 创建一个新的接收者信息
func NewReceiverInfo(baseID int64, data string) *ReceiverInfo {
	return &ReceiverInfo{
		BaseID: baseID,
		Data:   data,
	}
}
