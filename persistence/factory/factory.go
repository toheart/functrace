package factory

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toheart/functrace/domain"
	"github.com/toheart/functrace/persistence/memory"
	"github.com/toheart/functrace/persistence/sqlite"
)

// OpType 定义数据库操作类型
type OpType int

// 数据库操作类型常量
const (
	OpTypeInsert OpType = iota // 插入操作
	OpTypeUpdate               // 更新操作
)

// DBOperation 定义数据库操作
type DBOperation struct {
	OpType OpType        // 操作类型
	Query  string        // SQL查询语句
	Args   []interface{} // 查询参数
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DBType      string        // 数据库类型，例如 "sqlite"
	DSN         string        // 数据源名称
	MaxOpenConn int           // 最大打开连接数
	MaxIdleConn int           // 最大空闲连接数
	MaxIdleTime time.Duration // 连接最大空闲时间
}

// DatabaseType 数据库类型
type DatabaseType string

const (
	DBTypeSQLite DatabaseType = "sqlite"
	DBTypeMySQL  DatabaseType = "mysql"
	DBTypeMock   DatabaseType = "mock" // 添加Mock数据库类型
	// 可以添加更多数据库类型
)

// CreateRepositoryFactory 创建仓储工厂
func CreateRepositoryFactory(dbType string, logger *logrus.Logger) (domain.RepositoryFactory, error) {
	var factory domain.RepositoryFactory

	// 根据数据库类型返回对应的仓储工厂
	switch dbType {
	case "sqlite":
		factory = sqlite.NewSQLiteDatabase(logger)
	case "mock":
		factory = memory.NewMockDatabase(logger)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	// 初始化数据库
	if err := factory.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize database failed: %w", err)
	}

	return factory, nil
}

// CloseFactory 关闭指定仓储工厂并释放资源
func CloseFactory(factory domain.RepositoryFactory) error {
	if factory == nil {
		return nil
	}

	// 查找对应的数据库实例
	err := factory.Close()

	return err
}
