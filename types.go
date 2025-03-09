package functrace

type TraceData struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	GID            uint64 `json:"gid"`
	Indent         int    `json:"indent"`
	Params         string `json:"params"`
	TimeCost       string `json:"timeCost"`
	ParentFuncName string `json:"parentFuncname"`
	CreatedAt      string `json:"createdAt"`
	Seq            string `json:"seq"`
}
