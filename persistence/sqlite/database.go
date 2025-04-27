package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain"
)

// 确保SQLiteDatabase实现了IDatabase接口
var _ domain.RepositoryFactory = (*SQLiteDatabase)(nil)

// SQLiteDatabase SQLite数据库实现
type SQLiteDatabase struct {
	goroutineRepository domain.GoroutineRepository
	traceRepository     domain.TraceRepository
	paramRepository     domain.ParamRepository
	db                  *sql.DB
	logger              *logrus.Logger
}

// NewSQLiteDatabase 创建新的SQLite数据库
func NewSQLiteDatabase(logger *logrus.Logger) domain.RepositoryFactory {
	s := &SQLiteDatabase{
		db:     nil,
		logger: logger,
	}

	return s
}

// Initialize 初始化数据库
func (s *SQLiteDatabase) Initialize() error {
	// 创建数据库连接
	dbPath := findAvailableDBName()
	var err error
	s.logger.Infof("opening db: %s", dbPath)
	s.db, err = sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath))
	if err != nil {
		return fmt.Errorf("can't open db: %w", err)
	}

	// 设置连接参数
	s.db.SetMaxOpenConns(50)
	s.db.SetMaxIdleConns(10)
	s.db.SetConnMaxIdleTime(30 * time.Second)

	// 测试连接
	if err := s.db.Ping(); err != nil {
		return fmt.Errorf("can't ping db: %w", err)
	}

	// 创建表
	if err := s.createTablesAndIndexes(); err != nil {
		return fmt.Errorf("can't create tables and indexes: %w", err)
	}

	return nil
}

// createTables 创建数据表
func (s *SQLiteDatabase) createTablesAndIndexes() error {
	tables := []string{
		SQLCreateTraceTable,
		SQLCreateGoroutineTable,
		SQLCreateParamTable,

		// 创建索引
		SQLCreateGIDIndex,
		SQLCreateParentIndex,
		SQLCreateParamTraceIndex,
		SQLCreateParamBaseIndex,
	}

	for _, table := range tables {
		if _, err := s.db.Exec(table); err != nil {
			return fmt.Errorf("can't exec sql: %s, %w", table, err)
		}
	}
	s.goroutineRepository = NewGoroutineRepository(s.db)
	s.traceRepository = NewTraceRepository(s.db)
	s.paramRepository = NewParamRepository(s.db)

	return nil
}

// Close 关闭数据库连接
func (s *SQLiteDatabase) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLiteDatabase) GetGoroutineRepository() domain.GoroutineRepository {
	return s.goroutineRepository
}

func (s *SQLiteDatabase) GetTraceRepository() domain.TraceRepository {
	return s.traceRepository
}

func (s *SQLiteDatabase) GetParamRepository() domain.ParamRepository {
	return s.paramRepository
}

// findAvailableDBName 查找可用的数据库文件名
func findAvailableDBName() string {
	execName, err := os.Executable()
	if err != nil {
		execName = "default"
	}
	execName = filepath.Base(execName)
	currentTime := time.Now().Format("20060102150405")
	return fmt.Sprintf(DBFileNameFormat, execName, currentTime)
}
