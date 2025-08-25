package trace

import (
	"sync"
)

// TraceSession 表示单个goroutine的独享状态
type TraceSession struct {
	mu      sync.Mutex
	gid     uint64
	indent  int
	parents map[int]int64

	// 会话内数据队列与转发器
	opCh          chan *DataOp
	forwarderOnce sync.Once
	forwarderDone chan struct{}

	// 关联的实例与关闭控制
	inst   *TraceInstance
	closed bool
	wg     sync.WaitGroup
}

func NewTraceSession(gid uint64) *TraceSession {
	return &TraceSession{
		gid:           gid,
		indent:        0,
		parents:       make(map[int]int64),
		opCh:          make(chan *DataOp, 256),
		forwarderDone: make(chan struct{}),
	}
}

// PrepareEnter 计算进入时所需的信息，并更新本会话状态
func (s *TraceSession) PrepareEnter(inst *TraceInstance) (indent int, parentId int64, traceId int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 记录实例，用于入队退化发送
	if s.inst == nil {
		s.inst = inst
	}

	// 读取当前缩进与父ID
	indent = s.indent
	parentId = s.parents[indent-1]

	// 生成全局唯一traceId
	traceId = inst.nextTraceID(s.gid)

	// 更新父映射与缩进
	s.parents[indent] = traceId
	s.indent++

	return indent, parentId, traceId
}

// OnExit 在退出时回退缩进并返回退出前的缩进值
func (s *TraceSession) OnExit() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	indent := s.indent
	// 回退缩进，防止变为负数
	if s.indent > 0 {
		s.indent--
		// 清理当前层的父ID映射
		delete(s.parents, s.indent)
	} else {
		// 如果已经是0或负数，重置状态
		s.indent = 0
		s.parents = make(map[int]int64)
	}
	return indent
}

// EnsureForwarder 确保为该会话启动一个转发器，将会话内的操作转发到实例的发送通道
func (s *TraceSession) EnsureForwarder(inst *TraceInstance) {
	s.mu.Lock()
	if s.inst == nil {
		s.inst = inst
	}
	s.mu.Unlock()

	s.forwarderOnce.Do(func() {
		go func() {
			defer close(s.forwarderDone)
			for op := range s.opCh {
				// 统一从实例路径发送，内部会根据模式决定异步/同步，且不丢弃
				s.inst.sendOp(op)
			}
		}()
	})
}

// Enqueue 将操作入队（会话内有界通道）。满则阻塞当前goroutine，确保不丢弃。
func (s *TraceSession) Enqueue(op *DataOp) {
	// 快速路径：若已关闭，直接同步发送，避免向已关闭通道发送
	s.mu.Lock()
	closed := s.closed
	if !closed {
		// 计入在途，确保Close能够等待安全关闭
		s.wg.Add(1)
	}
	ch := s.opCh
	inst := s.inst
	s.mu.Unlock()

	if closed {
		// 会话关闭后，退化为直接发送，保证不丢弃
		inst.sendOp(op)
		return
	}

	// 发送到会话队列，发送完成后减少在途计数
	ch <- op
	s.wg.Done()
}

// Close 关闭会话队列并等待转发器退出（确保已将所有操作转发出去）
func (s *TraceSession) Close() {
	// 标记关闭，阻止新的入队进入通道
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	// 等待在途入队完成
	s.wg.Wait()
	// 关闭通道并等待转发器退出
	close(s.opCh)
	<-s.forwarderDone
}
