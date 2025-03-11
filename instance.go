package functrace

import (
	"database/sql"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	once        sync.Once
	singleTrace *TraceInstance
	currentNow  time.Time
)

// TraceInstance 是管理函数跟踪的单例结构体
type TraceInstance struct {
	sync.Mutex
	globalId     atomic.Int64
	indentations map[uint64]*TraceIndent
	log          *slog.Logger
	db           *sql.DB            // 数据库连接
	closed       bool               // 标志位表示是否已关闭
	updateChans  []chan dbOperation // 多个通道用于异步数据库操作
	chanCount    int                // 通道数量
}

// NewTraceInstance 初始化并返回 TraceInstance 的单例实例
func NewTraceInstance() *TraceInstance {
	once.Do(func() {
		// 初始化 TraceInstance
		initTraceInstance()

		// 初始化数据库
		if err := initDatabase(); err != nil {
			singleTrace.log.Error("数据库初始化失败", "error", err)
			return
		}

		// 启动异步处理数据库操作的协程
		go singleTrace.processDBUpdate()
	})
	return singleTrace
}

// initTraceInstance 初始化 TraceInstance 实例
func initTraceInstance() {
	// 从环境变量获取通道数量，默认为 DefaultChannelCount
	chanCount := DefaultChannelCount
	if chanCountStr := os.Getenv(EnvTraceChannelCount); chanCountStr != "" {
		if count, err := strconv.Atoi(chanCountStr); err == nil && count > 0 {
			chanCount = count
		}
	}

	// 初始化通道
	updateChans := make([]chan dbOperation, chanCount)
	for i := 0; i < chanCount; i++ {
		updateChans[i] = make(chan dbOperation, DefaultChannelBufferSize)
	}

	// 创建 TraceInstance
	singleTrace = &TraceInstance{
		indentations: make(map[uint64]*TraceIndent),
		log:          initializeLogger(),
		closed:       false,
		updateChans:  updateChans,
		chanCount:    chanCount,
	}
}

// initializeLogger 初始化日志记录器
func initializeLogger() *slog.Logger {
	log := slog.New(slog.NewTextHandler(&lumberjack.Logger{
		Filename:  LogFileName,
		LocalTime: true,
		Compress:  true,
	}, nil))
	return log
}

// initTraceIndentIfNeeded 如果需要，初始化 TraceIndent
func (t *TraceInstance) initTraceIndentIfNeeded(id uint64) {
	t.Lock()
	defer t.Unlock()

	if _, exists := t.indentations[id]; !exists {
		t.indentations[id] = &TraceIndent{
			Indent:      0,
			ParentFuncs: make(map[int]int64),
		}
	}
}

// Close 关闭数据库连接并释放资源
func (t *TraceInstance) Close() error {
	t.Lock()
	defer t.Unlock()

	// 如果已经关闭，直接返回
	if t.closed {
		return nil
	}

	// 标记为已关闭
	t.closed = true

	// 关闭所有通道
	for _, updateChan := range t.updateChans {
		close(updateChan)
	}

	// 关闭数据库连接
	if t.db != nil {
		return t.db.Close()
	}

	return nil
}

// CloseTraceInstance 关闭单例跟踪实例
func CloseTraceInstance() error {
	if singleTrace != nil {
		return singleTrace.Close()
	}
	return nil
}
