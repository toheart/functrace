package trace

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/domain/model"
	objDump "github.com/toheart/functrace/objectdump"
	"github.com/toheart/functrace/persistence/factory"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	once        sync.Once
	instance    *TraceInstance
	currentNow  time.Time
	stopMonitor chan struct{} // 停止监控的信号通道
)

// 存储仓储工厂的全局实例
var repositoryFactory domain.RepositoryFactory

// TraceIndent 存储函数调用的缩进信息和父函数名称
type TraceIndent struct {
	Indent      int           // 当前缩进级别
	ParentFuncs map[int]int64 // 每一层当前父函数ID
}

// GoroutineInfo 协程信息
type GoroutineInfo struct {
	ID             uint64 `json:"id"`             // 自增ID
	OriginGID      uint64 `json:"originGid"`      // 原始Goroutine ID
	LastUpdateTime string `json:"lastUpdateTime"` // 最后更新时间
}

// DataOp 数据操作
type DataOp struct {
	OpType OpType
	Arg    interface{}
}

// TraceInstance 是管理函数跟踪的单例结构体
type TraceInstance struct {
	sync.RWMutex
	globalId         atomic.Int64
	gGroutineId      atomic.Uint64
	gParamId         atomic.Int64
	indentations     map[uint64]*TraceIndent
	log              *logrus.Logger
	closed           bool                      // 标志位表示是否已关闭
	GoroutineRunning map[uint64]*GoroutineInfo // 管理运行中的goroutine, key为gid, value为数据库id

	paramCacheLock sync.RWMutex
	OpChan         chan *DataOp
	dataClose      chan struct{}
	config         *Config // 统一的配置管理

	// 内存监控器
	memoryMonitor *MemoryMonitor
	// TTL缓存管理器
	ttlManager *TTLCacheManager
}

// NewTraceInstance 初始化并返回 TraceInstance 的单例实例
func NewTraceInstance() *TraceInstance {
	once.Do(func() {
		currentNow = time.Now() // 记录启动时间
		initTraceInstance()
		instance.log.Info("init TraceInstance success")
		// 初始化数据库
		if err := initDatabase(); err != nil {
			instance.log.WithFields(logrus.Fields{"error": err}).Error("init database failed")
			return
		}
		instance.log.Info("init database success")
		instance.log.WithFields(logrus.Fields{"config": instance.config.String()}).Info("trace config initialized")
		instance.log.WithFields(logrus.Fields{"mode": instance.config.ParamStoreMode}).Info("param store mode initialized")

		// 启动协程监控
		go instance.monitorGoroutines()
		instance.log.Info("start goroutine monitor")

		// 根据参数存储模式决定是否启动相关服务
		if instance.config.ParamStoreMode == ParamStoreModeAll {
			// 启动TTL缓存管理器
			instance.ttlManager.Start()
		}
		if instance.config.ParamStoreMode != ParamStoreModeNone {
			// 启动内存监控器
			instance.memoryMonitor.Start()
			instance.log.WithFields(logrus.Fields{
				"memory_limit":   humanReadableBytes(instance.config.MemoryLimit),
				"check_interval": instance.config.MemoryCheckInterval,
			}).Info("memory monitor started for 'all' parameter store mode")
		}

		// 如果是异步模式，启动OpChan处理
		if instance.config.InsertMode == AsyncMode {
			go instance.StartOpChan()
			instance.log.Info("start op chan with async mode")
		} else {
			instance.log.Info("running in sync mode, op chan not started")
		}
	})
	return instance
}

// initTraceInstance 初始化 TraceInstance 实例
func initTraceInstance() {
	// 初始化停止监控通道
	stopMonitor = make(chan struct{})

	// 创建配置实例
	config := NewConfig()

	// 创建 TraceInstance
	instance = &TraceInstance{
		indentations:     make(map[uint64]*TraceIndent),
		log:              initializeLogger(),
		closed:           false,
		GoroutineRunning: make(map[uint64]*GoroutineInfo),
		OpChan:           make(chan *DataOp, 50),
		dataClose:        make(chan struct{}),
		config:           config,
	}

	// 初始化TTL缓存管理器
	instance.ttlManager = NewTTLCacheManager(instance.log)

	// 初始化内存监控器
	instance.memoryMonitor = NewMemoryMonitor(
		config.MemoryLimit,
		time.Duration(config.MemoryCheckInterval)*time.Second,
		instance.log,
	)
	// 设置spew配置
	instance.SetSpewConfig()
}

func (t *TraceInstance) StartOpChan() {
	p := pool.New().WithMaxGoroutines(50)
	for op := range t.OpChan {
		tmpOp := op
		p.Go(func() {
			switch tmpOp.OpType {
			case OpTypeInsert:
				t.insertOp(tmpOp)
			case OpTypeUpdate:
				t.updateOp(tmpOp)
			}
		})
	}
	p.Wait()
	t.dataClose <- struct{}{}
}

func (t *TraceInstance) insertOp(op *DataOp) {
	switch op.Arg.(type) {
	case *model.TraceData:
		t.saveTraceData(op.Arg.(*model.TraceData))
	case *model.ParamStoreData:
		t.storeParam(op.Arg.(*model.ParamStoreData))
	case *model.GoroutineTrace:
		t.saveGoroutineTrace(op.Arg.(*model.GoroutineTrace))
	}
}

func (t *TraceInstance) updateOp(op *DataOp) {
	switch op.Arg.(type) {
	case *model.TraceData:
		t.updateTraceData(op.Arg.(*model.TraceData))
	case *model.GoroutineTrace:
		t.updateGoroutineTimeCost(op.Arg.(*model.GoroutineTrace))
	}
}

// executeOp 直接执行数据库操作
func (t *TraceInstance) executeOp(op *DataOp) {
	switch op.OpType {
	case OpTypeInsert:
		t.insertOp(op)
	case OpTypeUpdate:
		t.updateOp(op)
	}
}

// initializeLogger 初始化日志记录器
func initializeLogger() *logrus.Logger {
	// 创建新的logrus实例
	log := logrus.New()

	// 配置日志格式为文本格式
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		DisableColors:    false,
		DisableTimestamp: false,
	})

	// 默认删除旧的日志文件
	if err := os.Remove(LogFileName); err != nil && !os.IsNotExist(err) {
		// 如果删除失败且不是因为文件不存在，记录警告
		log.Warnf("Failed to clear log file %s: %v", LogFileName, err)
	} else {
		log.Infof("Cleared log file: %s", LogFileName)
	}

	// 配置日志输出到lumberjack用于日志轮转
	logWriter := &lumberjack.Logger{
		Filename:  LogFileName,
		LocalTime: true,
		Compress:  true,
	}
	log.SetOutput(logWriter)

	// 设置日志级别
	log.SetLevel(logrus.InfoLevel)

	return log
}

// InitTraceIndentIfNeeded 如果需要，初始化 TraceIndent
func (t *TraceInstance) InitTraceIndentIfNeeded(id uint64) {
	t.Lock()
	defer t.Unlock()

	if _, exists := t.indentations[id]; !exists {
		t.indentations[id] = &TraceIndent{
			Indent:      0,
			ParentFuncs: make(map[int]int64),
		}
	}
}

// getAllGoroutineIDs 获取当前所有运行中的协程ID
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

	// 停止TTL缓存管理器
	if t.config.ParamStoreMode == ParamStoreModeAll {
		t.ttlManager.Stop()
	}

	// 停止内存监控器
	if t.memoryMonitor != nil && t.memoryMonitor.IsEnabled() {
		t.memoryMonitor.Stop()
		t.log.Info("memory monitor stopped")
	}

	// 如果是异步模式，关闭OpChan
	if t.config.InsertMode == AsyncMode {
		close(t.OpChan)
		<-t.dataClose
	}

	// 关闭数据库连接
	return CloseDatabase()
}

// initDatabase 初始化数据库
func initDatabase() error {
	// 创建仓储工厂
	var err error
	repositoryFactory, err = factory.CreateRepositoryFactory(instance.config.DBType, instance.log)
	if err != nil {
		return err
	}

	// 初始化仓储工厂
	if err := repositoryFactory.Initialize(); err != nil {
		return err
	}

	return nil
}

// CloseDatabase 关闭数据库连接
func CloseDatabase() error {
	if repositoryFactory != nil {
		return factory.CloseFactory(repositoryFactory)
	}
	return nil
}

// GetTraceInstance 获取跟踪实例
func GetTraceInstance() *TraceInstance {
	return instance
}

// GetLogger 获取日志实例
func (t *TraceInstance) GetLogger() *logrus.Logger {
	return t.log
}

// GetRepositoryFactory 获取仓储工厂
func GetRepositoryFactory() domain.RepositoryFactory {
	return repositoryFactory
}

func (t *TraceInstance) saveTraceData(trace *model.TraceData) {
	t.log.WithFields(logrus.Fields{"trace": trace}).Info("save trace data")
	_, err := repositoryFactory.GetTraceRepository().SaveTrace(trace)
	if err != nil {
		t.log.WithFields(logrus.Fields{"error": err, "trace": trace}).Error("save trace data failed")
	}
}

func (t *TraceInstance) updateTraceData(trace *model.TraceData) {
	err := repositoryFactory.GetTraceRepository().UpdateTraceTimeCost(trace.ID, trace.TimeCost)
	if err != nil {
		// 将数据重新插回队列
		t.sendOp(&DataOp{
			OpType: OpTypeUpdate,
			Arg:    trace,
		})
		t.log.WithFields(logrus.Fields{"error": err, "trace": trace}).Error("update trace data failed, requeue")
	}
}

func (t *TraceInstance) sendOp(op *DataOp) {
	// 根据插入模式决定是同步执行还是通过通道异步执行
	if t.config.InsertMode == SyncMode {
		t.executeOp(op)
	} else {
		t.OpChan <- op
	}
}

// GetParamStoreMode 获取当前的参数存储模式
func (t *TraceInstance) GetParamStoreMode() string {
	return t.config.ParamStoreMode
}

// IsParamStoreEnabled 检查是否启用了参数存储
func (t *TraceInstance) IsParamStoreEnabled() bool {
	return t.config.ParamStoreMode != ParamStoreModeNone
}

// IsParamStoreAll 检查是否为全保存模式
func (t *TraceInstance) IsParamStoreAll() bool {
	return t.config.ParamStoreMode == ParamStoreModeAll
}

// GetIgnoreNames 获取忽略的函数名列表
func (t *TraceInstance) GetIgnoreNames() []string {
	return t.config.IgnoreNames
}

// GetSpewConfig 获取spew配置
func (t *TraceInstance) SetSpewConfig() {
	objDump.SetGlobalConfig(t.config.CreateSpewConfig())
}
