package trace

import (
	"sync"
)

// SessionRegistry 管理 gid -> TraceSession 的映射
type SessionRegistry struct {
	mu    sync.RWMutex
	table map[uint64]*TraceSession
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{table: make(map[uint64]*TraceSession)}
}

func (r *SessionRegistry) GetOrCreate(gid uint64) *TraceSession {
	r.mu.RLock()
	if s, ok := r.table[gid]; ok {
		r.mu.RUnlock()
		return s
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.table[gid]; ok {
		return s
	}
	s := NewTraceSession(gid)
	r.table[gid] = s
	return s
}

func (r *SessionRegistry) Remove(gid uint64) {
	r.mu.Lock()
	delete(r.table, gid)
	r.mu.Unlock()
}
