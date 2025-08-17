package trace

import (
	"hash/fnv"
	"sync/atomic"
)

// IDGenerator 统一的ID生成接口
type IDGenerator interface {
	NextTraceID(shardKey uint64) int64
	NextParamID(shardKey uint64) int64
}

// StripedIDGenerator 基于分片计数器的ID生成器
// 将竞争分散到N个分片：id = shardIndex + counter*N
type StripedIDGenerator struct {
	shardCount  uint64
	traceShards []atomic.Int64
	paramShards []atomic.Int64
}

// NewStripedIDGenerator 创建一个新的分片ID生成器
func NewStripedIDGenerator(shardCount uint64) *StripedIDGenerator {
	if shardCount == 0 {
		shardCount = 32
	}
	gen := &StripedIDGenerator{
		shardCount:  shardCount,
		traceShards: make([]atomic.Int64, shardCount),
		paramShards: make([]atomic.Int64, shardCount),
	}
	return gen
}

// shardIndex 依据键选择固定分片
func (g *StripedIDGenerator) shardIndex(key uint64) uint64 {
	// 使用FNV哈希增加分布均匀性，同时支持任意key
	h := fnv.New64a()
	var b [8]byte
	b[0] = byte(key >> 56)
	b[1] = byte(key >> 48)
	b[2] = byte(key >> 40)
	b[3] = byte(key >> 32)
	b[4] = byte(key >> 24)
	b[5] = byte(key >> 16)
	b[6] = byte(key >> 8)
	b[7] = byte(key)
	_, _ = h.Write(b[:])
	return h.Sum64() % g.shardCount
}

// NextTraceID 返回下一个全局唯一的TraceID
func (g *StripedIDGenerator) NextTraceID(shardKey uint64) int64 {
	idx := g.shardIndex(shardKey)
	n := g.traceShards[idx].Add(1)
	return int64(n*int64(g.shardCount) + int64(idx))
}

// NextParamID 返回下一个全局唯一的ParamID
func (g *StripedIDGenerator) NextParamID(shardKey uint64) int64 {
	idx := g.shardIndex(shardKey)
	n := g.paramShards[idx].Add(1)
	return int64(n*int64(g.shardCount) + int64(idx))
}
