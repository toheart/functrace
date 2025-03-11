package functrace

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/glebarez/go-sqlite" // 引入 sqlite3 驱动
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
)

// initDatabase 初始化数据库连接和表结构
func initDatabase() error {
	var err error
	currentNow = time.Now()

	// 查找可用的数据库文件名
	dbName, err := findAvailableDBName()
	if err != nil {
		return fmt.Errorf("can't find available db name: %w", err)
	}

	// 打开数据库连接
	singleTrace.db, err = sql.Open("sqlite", dbName)
	if err != nil {
		return fmt.Errorf("can't open db: %w", err)
	}

	// 测试数据库连接
	if err = singleTrace.db.Ping(); err != nil {
		return fmt.Errorf("can't ping db: %w", err)
	}

	// 创建表和索引
	if err = createTablesAndIndexes(singleTrace.db); err != nil {
		return fmt.Errorf("can't create tables and indexes: %w", err)
	}

	return nil
}

// findAvailableDBName 查找可用的数据库文件名
func findAvailableDBName() (string, error) {
	execName, err := os.Executable()
	if err != nil {
		execName = "default"
	}
	index := 0
	for {
		dbName := fmt.Sprintf(DBFileNameFormat, execName, index)
		if _, err := os.Stat(dbName); os.IsNotExist(err) {
			return dbName, nil
		}
		index++
		// 防止无限循环
		if index > 1000 {
			return "", fmt.Errorf("can't find available db name")
		}
	}
}

// createTablesAndIndexes 创建数据库表和索引
func createTablesAndIndexes(db *sql.DB) error {
	// 创建表
	if _, err := db.Exec(SQLCreateTable); err != nil {
		return fmt.Errorf("can't create table: %w", err)
	}

	// 创建索引
	if _, err := db.Exec(SQLCreateGIDIndex); err != nil {
		return fmt.Errorf("can't create GID index: %w", err)
	}

	if _, err := db.Exec(SQLCreateParentIndex); err != nil {
		return fmt.Errorf("can't create ParentId index: %w", err)
	}

	return nil
}

// processDBUpdate 处理异步数据库操作
func (t *TraceInstance) processDBUpdate() {
	wg := conc.NewWaitGroup()

	// 为每个通道启动一个处理协程
	for i := 0; i < t.chanCount; i++ {
		chanIndex := i // 捕获变量
		wg.Go(func() {
			t.processChannel(chanIndex)
		})
	}

	wg.Wait()
}

// processChannel 处理单个通道的数据库操作
func (t *TraceInstance) processChannel(chanIndex int) {
	pool := pool.New().WithMaxGoroutines(DefaultPoolSize)

	for op := range t.updateChans[chanIndex] {
		operation := op // 捕获变量
		pool.Go(func() {
			t.executeDBOperation(operation)
		})
	}
}

// executeDBOperation 执行单个数据库操作
func (t *TraceInstance) executeDBOperation(op dbOperation) {
	var result sql.Result
	var err error

	// 根据操作类型执行不同的数据库操作
	switch op.opType {
	case OpTypeInsert:
		result, err = t.db.Exec(op.query, op.args...)
		if err != nil {
			t.log.Error("can't insert data", "error", err, "query", op.query)
			return
		}
		t.log.Info("insert data success", "result", result)

	case OpTypeUpdate:
		result, err = t.db.Exec(op.query, op.args...)
		if err != nil {
			t.log.Error("can't update data", "error", err, "query", op.query)
			return
		}
		t.log.Info("update data success", "result", result)

	default:
		t.log.Error("unknown operation type", "opType", op.opType)
	}
}

// sendDBOperation 发送数据库操作到对应的通道
func (t *TraceInstance) sendDBOperation(id uint64, opType OpType, query string, args []interface{}) {
	// 使用通道索引，根据goroutine ID进行哈希分配
	chanIndex := int(id % uint64(t.chanCount))

	// 发送操作到对应的通道
	t.updateChans[chanIndex] <- dbOperation{
		opType: opType,
		query:  query,
		args:   args,
	}
}
