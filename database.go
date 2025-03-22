package functrace

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc"
	_ "modernc.org/sqlite" // 引入 sqlite3 驱动
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
	singleTrace.log.WithFields(logrus.Fields{"dbName": dbName}).Info("found dbName")

	// 打开数据库连接
	singleTrace.db, err = sql.Open("sqlite", fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL", dbName))
	if err != nil {
		return fmt.Errorf("can't open db: %w", err)
	}
	singleTrace.db.SetMaxOpenConns(50)
	singleTrace.db.SetMaxIdleConns(10)
	singleTrace.db.SetConnMaxIdleTime(30 * time.Second)

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
	execName = filepath.Base(execName)
	currentTime := time.Now().Format("20060102150405")
	dbName := fmt.Sprintf(DBFileNameFormat, execName, currentTime)

	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		return dbName, nil
	}
	return "", fmt.Errorf("can't find available db name")
}

// createTablesAndIndexes 创建数据库表和索引
func createTablesAndIndexes(db *sql.DB) error {
	// 创建跟踪表
	if _, err := db.Exec(SQLCreateTable); err != nil {
		return fmt.Errorf("can't create trace table: %w", err)
	}

	// 创建goroutine表
	if _, err := db.Exec(SQLCreateGoroutineTable); err != nil {
		return fmt.Errorf("can't create goroutine table: %w", err)
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

	t.dbClosed <- struct{}{}
}

// processChannel 处理单个通道的数据库操作
func (t *TraceInstance) processChannel(chanIndex int) {
	for op := range t.updateChans[chanIndex] {
		t.executeDBOperation(op)
	}
}

// executeDBOperation 执行数据库操作
func (t *TraceInstance) executeDBOperation(op dbOperation) {
	switch op.opType {
	case OpTypeInsert:
		// 执行插入操作
		result, err := t.db.Exec(op.query, op.args...)
		if err != nil {
			t.log.WithFields(logrus.Fields{
				"error": err,
				"query": op.query,
			}).Error("can't insert data")
			return
		}
		t.log.WithFields(logrus.Fields{"result": result}).Info("insert data success")

	case OpTypeUpdate:
		// 执行更新操作
		result, err := t.db.Exec(op.query, op.args...)
		if err != nil {
			t.log.WithFields(logrus.Fields{
				"error": err,
				"query": op.query,
			}).Error("can't update data")
			return
		}
		t.log.WithFields(logrus.Fields{"result": result}).Info("update data success")

	default:
		t.log.WithFields(logrus.Fields{"opType": op.opType}).Error("unknown operation type")
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
