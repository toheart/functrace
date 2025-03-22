package functrace

import (
	"database/sql"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	once        sync.Once
	singleTrace *TraceInstance
	currentNow  time.Time
	stopMonitor chan struct{} // 停止监控的信号通道
)

// TraceInstance 是管理函数跟踪的单例结构体
type TraceInstance struct {
	sync.RWMutex
	globalId         atomic.Int64
	gGroutineId      atomic.Uint64
	indentations     map[uint64]*TraceIndent
	log              *slog.Logger
	db               *sql.DB                   // 数据库连接
	closed           bool                      // 标志位表示是否已关闭
	dbClosed         chan struct{}             // 标志位表示数据库是否已关闭
	updateChans      []chan dbOperation        // 多个通道用于异步数据库操作
	chanCount        int                       // 通道数量
	GoroutineRunning map[uint64]*GoroutineInfo // 管理运行中的goroutine, key为gid, value为数据库id

	IgnoreNames map[string]struct{}
	spewConfig  *spew.ConfigState
}

// NewTraceInstance 初始化并返回 TraceInstance 的单例实例
func NewTraceInstance() *TraceInstance {
	once.Do(func() {
		initTraceInstance()
		singleTrace.log.Info("init TraceInstance success")
		// 初始化数据库
		if err := initDatabase(); err != nil {
			singleTrace.log.Error("init database failed", "error", err)
			return
		}
		singleTrace.log.Info("init database success")
		singleTrace.log.Info("spew config", "config", spew.Config)

		// 启动异步处理数据库操作的协程
		go singleTrace.processDBUpdate()

		// 启动goroutine监控
		go singleTrace.monitorGoroutines()
		singleTrace.log.Info("goroutine monitor started")
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
	ignoreEnv := os.Getenv(EnvIgnoreNames)
	var ignoreNames []string
	if ignoreEnv != "" {
		ignoreNames = strings.Split(ignoreEnv, ",")
	} else {
		ignoreNames = strings.Split(IgnoreNames, ",")
	}
	IgnoreNamesMap := make(map[string]struct{})
	for _, name := range ignoreNames {
		IgnoreNamesMap[name] = struct{}{}
	}

	// 初始化通道
	updateChans := make([]chan dbOperation, chanCount)
	for i := 0; i < chanCount; i++ {
		updateChans[i] = make(chan dbOperation, DefaultChannelBufferSize)
	}

	// 初始化停止监控通道
	stopMonitor = make(chan struct{})

	// 创建 TraceInstance
	singleTrace = &TraceInstance{
		indentations:     make(map[uint64]*TraceIndent),
		log:              initializeLogger(),
		closed:           false,
		dbClosed:         make(chan struct{}),
		updateChans:      updateChans,
		chanCount:        chanCount,
		GoroutineRunning: make(map[uint64]*GoroutineInfo),
		IgnoreNames:      IgnoreNamesMap,
		spewConfig: &spew.ConfigState{
			Indent:                  "  ",
			MaxDepth:                5,
			DisableMethods:          true,
			DisableCapacities:       true,
			DisablePointerAddresses: true,
			DisablePointerMethods:   true,
		},
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

func (t *TraceInstance) getAllGoroutineIDs() []int {
	buf := make([]byte, 1<<20) // 分配 1MB 缓冲区
	n := runtime.Stack(buf, true)
	stack := string(buf[:n])
	var ids []int

	for _, line := range strings.Split(stack, "\n") {
		if strings.HasPrefix(line, "goroutine ") {
			// 解析行如 "goroutine 123 [running]:"
			parts := strings.Fields(line)
			idStr := parts[1]
			id, err := strconv.Atoi(idStr)
			if err == nil {
				ids = append(ids, id)
			}
		}
	}
	return ids
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

	// 发送停止监控信号
	close(stopMonitor)

	// 关闭所有通道
	for _, updateChan := range t.updateChans {
		close(updateChan)
	}
	<-t.dbClosed

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
