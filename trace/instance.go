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
	"github.com/toheart/functrace/persistence/factory"
	"github.com/toheart/functrace/spew"
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

// receiverInfo 接收者信息
type receiverInfo struct {
	TraceID int64  // 跟踪ID
	BaseID  int64  // 基础参数ID
	Data    string // JSON格式的参数数据
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
	paramCache       map[string]*receiverInfo  // 管理参数缓存, key为值指针地址, value为参数缓存

	OpChan    chan *DataOp
	dataClose chan struct{}

	IgnoreNames []string
	spewConfig  *spew.ConfigState
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
		instance.log.WithFields(logrus.Fields{"config": spew.Config}).Info("spew config")

		// 启动协程监控
		go instance.monitorGoroutines()
		instance.log.Info("start goroutine monitor")
		go instance.StartOpChan()
		instance.log.Info("start op chan")
	})
	return instance
}

// initTraceInstance 初始化 TraceInstance 实例
func initTraceInstance() {
	// 从环境变量获取忽略名称
	ignoreEnv := os.Getenv(EnvIgnoreNames)
	var ignoreNames []string
	if ignoreEnv != "" {
		ignoreNames = strings.Split(ignoreEnv, ",")
	} else {
		ignoreNames = strings.Split(IgnoreNames, ",")
	}

	// 初始化停止监控通道
	stopMonitor = make(chan struct{})

	// 获取配置的最大深度
	maxDepth := DefaultMaxDepth
	if maxDepthStr := os.Getenv(EnvMaxDepth); maxDepthStr != "" {
		if count, err := strconv.Atoi(maxDepthStr); err == nil && count > 0 {
			maxDepth = count
		}
	}

	// 创建 TraceInstance
	instance = &TraceInstance{
		indentations:     make(map[uint64]*TraceIndent),
		log:              initializeLogger(),
		closed:           false,
		GoroutineRunning: make(map[uint64]*GoroutineInfo),
		paramCache:       make(map[string]*receiverInfo),
		IgnoreNames:      ignoreNames,
		OpChan:           make(chan *DataOp, 50),
		dataClose:        make(chan struct{}),

		spewConfig: &spew.ConfigState{
			MaxDepth:          maxDepth + 1, // 从业务角度，需要多一层
			DisableMethods:    true,
			DisableCapacities: true,
			EnableJSONOutput:  true,
			SkipNilValues:     true,
		},
	}
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
	close(t.dataClose)
	<-t.dataClose

	// 关闭数据库连接
	return CloseDatabase()
}

// initDatabase 初始化数据库
func initDatabase() error {
	// 从环境变量获取数据库类型，默认使用 sqlite
	dbType := os.Getenv(EnvDBType)
	if dbType == "" {
		dbType = "sqlite"
	}

	// 创建仓储工厂
	var err error
	repositoryFactory, err = factory.CreateRepositoryFactory(dbType, instance.log)
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
		t.log.WithFields(logrus.Fields{"error": err, "trace": trace}).Error("update trace data failed")
	}
}

func (t *TraceInstance) sendOp(op *DataOp) {
	t.OpChan <- op
}
