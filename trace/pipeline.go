package trace

import (
	"context"
	"time"

	"github.com/toheart/functrace/domain/model"
)

// TracePipeline 定义 trace 事件的管道接口（插入与更新）
type TracePipeline interface {
	// Insert 提交 trace 插入事件
	Insert(trace *model.TraceData)
	// Update 提交 trace 更新事件
	Update(trace *model.TraceData)
}

// ParamPipeline 定义参数批量入库管道接口
type ParamPipeline interface {
	// Enqueue 提交参数数据进入批量器
	Enqueue(p *model.ParamStoreData)
}

// GoroutinePipeline 定义 goroutine 插入与更新管道接口
type GoroutinePipeline interface {
	// Insert 保存 goroutine 记录
	Insert(g *model.GoroutineTrace)
	// Update 更新 goroutine 记录
	Update(g *model.GoroutineTrace)
}

// Pipelines 聚合三类数据的管道，统一生命周期管理
type Pipelines struct {
	ctx    context.Context
	cancel context.CancelFunc

	Trace     TracePipeline
	Param     ParamPipeline
	Goroutine GoroutinePipeline
}

// NewPipelines 创建管道骨架（默认使用空实现，后续逐步替换为真实实现)
func NewPipelines(parent context.Context, inst *TraceInstance) *Pipelines {
	ctx, cancel := context.WithCancel(parent)
	p := &Pipelines{
		ctx:    ctx,
		cancel: cancel,
	}
	// Trace 管道（分片管理）
	shardNum := inst.config.TraceShardNum
	if shardNum <= 0 {
		shardNum = 16
	}
	p.Trace = newTracePipeline(ctx, shardNum)
	p.Param = newParamPipeline(ctx)
	p.Goroutine = newGoroutinePipeline(ctx)
	return p
}

// Start 启动所有子管道（骨架版本为空实现）
func (p *Pipelines) Start() {
	// 未来：在此启动各子管道的后台协程
}

// Stop 停止所有子管道（骨架版本：仅取消 context）
func (p *Pipelines) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

// ---- Trace 分片管道实现 ----

type tracePipeline struct {
	ctx    context.Context
	shards []*tpShard
}

func newTracePipeline(ctx context.Context, shardNum int) *tracePipeline {
	tp := &tracePipeline{
		ctx:    ctx,
		shards: make([]*tpShard, shardNum),
	}
	for i := 0; i < shardNum; i++ {
		sh := newTpShard(i)
		tp.shards[i] = sh
		sh.start(ctx)
	}
	return tp
}

func (t *tracePipeline) shardIndex(traceID int64) int {
	if len(t.shards) == 0 {
		return 0
	}
	if traceID < 0 {
		traceID = -traceID
	}
	return int(uint64(traceID) % uint64(len(t.shards)))
}

func (t *tracePipeline) Insert(td *model.TraceData) {
	idx := t.shardIndex(td.ID)
	sh := t.shards[idx]
	if sh == nil || sh.inCh == nil {
		_, _ = repositoryFactory.GetTraceRepository().SaveTrace(td)
		return
	}
	select {
	case sh.inCh <- td:
		// ok
	default:
		_, _ = repositoryFactory.GetTraceRepository().SaveTrace(td)
	}
}

func (t *tracePipeline) Update(td *model.TraceData) {
	idx := t.shardIndex(td.ID)
	sh := t.shards[idx]
	evt := tpUpdateEvt{id: td.ID, timeCost: td.TimeCost}
	if sh == nil || sh.inCh == nil {
		_ = repositoryFactory.GetTraceRepository().UpdateTraceTimeCost(td.ID, td.TimeCost)
		return
	}
	select {
	case sh.inCh <- evt:
		// ok
	default:
		_ = repositoryFactory.GetTraceRepository().UpdateTraceTimeCost(td.ID, td.TimeCost)
	}
}

type tpShard struct {
	index       int
	inCh        chan interface{}
	insertedSet map[int64]struct{}
	retryQueue  []interface{}
	retryTimer  *time.Timer
}

func newTpShard(index int) *tpShard {
	return &tpShard{
		index:       index,
		inCh:        make(chan interface{}, 1024),
		insertedSet: make(map[int64]struct{}),
		retryQueue:  make([]interface{}, 0),
	}
}

func (s *tpShard) start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-s.inCh:
				if !ok {
					return
				}
				s.handleEvent(evt)
			case <-s.retryTimerC():
				s.drainRetry()
			}
		}
	}()
}

func (s *tpShard) retryTimerC() <-chan time.Time {
	if s.retryTimer != nil {
		return s.retryTimer.C
	}
	return make(<-chan time.Time)
}

func (s *tpShard) scheduleRetry(delay time.Duration) {
	if s.retryTimer == nil {
		s.retryTimer = time.NewTimer(delay)
		return
	}
	if !s.retryTimer.Stop() {
		select {
		case <-s.retryTimer.C:
		default:
		}
	}
	s.retryTimer.Reset(delay)
}

func (s *tpShard) drainRetry() {
	if len(s.retryQueue) == 0 {
		return
	}
	q := s.retryQueue
	s.retryQueue = nil
	for _, evt := range q {
		s.handleEvent(evt)
	}
	if len(s.retryQueue) > 0 {
		s.scheduleRetry(100 * time.Millisecond)
	}
}

type tpUpdateEvt struct {
	id       int64
	timeCost string
}

func (s *tpShard) handleEvent(evt interface{}) {
	switch e := evt.(type) {
	case *model.TraceData:
		if _, ok := s.insertedSet[e.ID]; ok {
			return
		}
		if _, err := repositoryFactory.GetTraceRepository().SaveTrace(e); err != nil {
			s.retryQueue = append(s.retryQueue, e)
			s.scheduleRetry(100 * time.Millisecond)
			return
		}
		s.insertedSet[e.ID] = struct{}{}
	case tpUpdateEvt:
		id := e.id
		if _, ok := s.insertedSet[id]; !ok {
			s.retryQueue = append(s.retryQueue, e)
			s.scheduleRetry(50 * time.Millisecond)
			return
		}
		if err := repositoryFactory.GetTraceRepository().UpdateTraceTimeCost(id, e.timeCost); err != nil {
			s.retryQueue = append(s.retryQueue, e)
			s.scheduleRetry(100 * time.Millisecond)
			return
		}
		delete(s.insertedSet, id)
	}
}

// ---- Param 批量器实现（迁移自 TraceInstance.startParamBatcher） ----

type paramPipeline struct {
	ctx  context.Context
	inCh chan *model.ParamStoreData
}

func newParamPipeline(ctx context.Context) *paramPipeline {
	p := &paramPipeline{
		ctx:  ctx,
		inCh: make(chan *model.ParamStoreData, 1000),
	}
	go p.loop()
	return p
}

func (p *paramPipeline) Enqueue(ps *model.ParamStoreData) {
	select {
	case p.inCh <- ps:
		// ok
	default:
		// 通道满：直接降级为单条写入
		_, _ = repositoryFactory.GetParamRepository().SaveParam(ps)
	}
}

func (p *paramPipeline) loop() {
	const (
		maxBatchSize  = 256
		flushInterval = 500 * time.Millisecond
	)
	batch := make([]*model.ParamStoreData, 0, maxBatchSize)
	timer := time.NewTimer(flushInterval)
	defer timer.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		b := batch
		batch = make([]*model.ParamStoreData, 0, maxBatchSize)
		if err := repositoryFactory.GetParamRepository().SaveParamsBatch(b); err != nil {
			for _, it := range b {
				_, _ = repositoryFactory.GetParamRepository().SaveParam(it)
			}
		}
	}
	for {
		select {
		case <-p.ctx.Done():
			flush()
			return
		case ps := <-p.inCh:
			batch = append(batch, ps)
			if len(batch) >= maxBatchSize {
				flush()
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(flushInterval)
			}
		case <-timer.C:
			flush()
			timer.Reset(flushInterval)
		}
	}
}

// ---- Goroutine 管道实现：串行处理，ctx 控制退出 ----

type goroutinePipeline struct {
	ctx  context.Context
	inCh chan goroutineEvt
}

type goroutineEvt struct {
	g        *model.GoroutineTrace
	isUpdate bool
}

func newGoroutinePipeline(ctx context.Context) *goroutinePipeline {
	p := &goroutinePipeline{
		ctx:  ctx,
		inCh: make(chan goroutineEvt, 512),
	}
	go p.loop()
	return p
}

func (g *goroutinePipeline) Insert(gt *model.GoroutineTrace) {
	evt := goroutineEvt{g: gt, isUpdate: false}
	select {
	case g.inCh <- evt:
		// ok
	default:
		_, _ = repositoryFactory.GetGoroutineRepository().SaveGoroutine(gt)
	}
}

func (g *goroutinePipeline) Update(gt *model.GoroutineTrace) {
	evt := goroutineEvt{g: gt, isUpdate: true}
	select {
	case g.inCh <- evt:
		// ok
	default:
		_ = repositoryFactory.GetGoroutineRepository().UpdateGoroutineTimeCost(gt.ID, gt.TimeCost, gt.IsFinished)
	}
}

func (g *goroutinePipeline) loop() {
	for {
		select {
		case <-g.ctx.Done():
			return
		case evt := <-g.inCh:
			if evt.isUpdate {
				_ = repositoryFactory.GetGoroutineRepository().UpdateGoroutineTimeCost(evt.g.ID, evt.g.TimeCost, evt.g.IsFinished)
			} else {
				_, _ = repositoryFactory.GetGoroutineRepository().SaveGoroutine(evt.g)
			}
		}
	}
}
