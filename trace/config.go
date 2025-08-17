package trace

import (
	"os"
	"strconv"
	"strings"

	objDump "github.com/toheart/functrace/objectdump"
)

// Config 统一的配置结构体
type Config struct {
	// 监控配置
	MonitorInterval int      // 协程监控间隔（秒）
	MaxDepth        int      // 最大跟踪深度
	IgnoreNames     []string // 忽略的函数名列表

	// 内存监控配置
	MemoryLimit         uint64 // 内存限制（字节）
	MemoryCheckInterval int    // 内存检查间隔（秒）

	// 数据库配置
	DBType     string // 数据库类型
	InsertMode string // 数据库插入模式：sync/async

	// 参数存储配置
	ParamStoreMode string // 参数存储模式：none/normal/all

	// 日志配置
	LogFileName string // 日志文件名

	// 对象序列化配置
	MaxElementsPerContainer int  // 单个容器最大递归元素数
	AllowUnexportedParams   bool // 是否允许序列化未导出字段
}

// configField 配置字段定义
type configField struct {
	envKey       string            // 环境变量键名
	defaultValue interface{}       // 默认值
	validator    func(string) bool // 验证函数（可选）
}

// 配置字段映射表
var configFields = map[string]configField{
	"MonitorInterval": {
		envKey:       EnvGoroutineMonitorInterval,
		defaultValue: DefaultMonitorInterval,
		validator: func(v string) bool {
			if i, err := strconv.Atoi(v); err == nil && i > 0 {
				return true
			}
			return false
		},
	},
	"MaxDepth": {
		envKey:       EnvMaxDepth,
		defaultValue: DefaultMaxDepth,
		validator: func(v string) bool {
			if i, err := strconv.Atoi(v); err == nil && i > 0 {
				return true
			}
			return false
		},
	},
	"IgnoreNames": {
		envKey:       EnvIgnoreNames,
		defaultValue: IgnoreNames,
	},
	"MemoryLimit": {
		envKey:       EnvMemoryLimit,
		defaultValue: uint64(DefaultMemoryLimit),
		validator: func(v string) bool {
			if i, err := strconv.ParseUint(v, 10, 64); err == nil && i > 0 {
				return true
			}
			return false
		},
	},
	"MemoryCheckInterval": {
		envKey:       "", // 暂时不提供环境变量配置，使用默认值
		defaultValue: DefaultMemoryCheckInterval,
		validator: func(v string) bool {
			if i, err := strconv.Atoi(v); err == nil && i > 0 {
				return true
			}
			return false
		},
	},
	"DBType": {
		envKey:       EnvDBType,
		defaultValue: "sqlite",
	},
	"InsertMode": {
		envKey:       EnvDBInsertMode,
		defaultValue: SyncMode,
		validator: func(v string) bool {
			return v == SyncMode || v == AsyncMode
		},
	},
	"ParamStoreMode": {
		envKey:       EnvParamStoreMode,
		defaultValue: ParamStoreModeAll,
		validator: func(v string) bool {
			return v == ParamStoreModeNone || v == ParamStoreModeNormal || v == ParamStoreModeAll
		},
	},
	"LogFileName": {
		envKey:       "",
		defaultValue: LogFileName,
	},
	"MaxElementsPerContainer": {
		defaultValue: 20,
		validator: func(v string) bool {
			if i, err := strconv.Atoi(v); err == nil && i > 0 {
				return true
			}
			return false
		},
	},
	"AllowUnexportedParams": {
		envKey:       "FUNCTRACE_ALLOW_UNEXPORTED",
		defaultValue: true,
		validator: func(v string) bool {
			_, err := strconv.ParseBool(v)
			return err == nil
		},
	},
}

// NewConfig 创建新的配置实例
func NewConfig() *Config {
	config := &Config{
		LogFileName:             LogFileName,
		MaxElementsPerContainer: 20,
	}

	// 加载所有配置
	config.loadFromEnv()

	return config
}

// loadFromEnv 从环境变量加载配置
func (c *Config) loadFromEnv() {
	// 监控间隔
	c.MonitorInterval = c.getIntEnv("MonitorInterval")

	// 最大深度
	c.MaxDepth = c.getIntEnv("MaxDepth")

	// 忽略的函数名列表
	c.IgnoreNames = c.getStringSliceEnv("IgnoreNames")

	// 内存限制
	c.MemoryLimit = c.getUint64Env("MemoryLimit")

	// 内存检查间隔
	c.MemoryCheckInterval = c.getIntEnv("MemoryCheckInterval")

	// 数据库类型
	c.DBType = c.getStringEnv("DBType")

	// 插入模式
	c.InsertMode = c.getStringEnv("InsertMode")

	// 参数存储模式
	c.ParamStoreMode = c.getStringEnv("ParamStoreMode")

	c.MaxElementsPerContainer = c.getIntEnv("MaxElementsPerContainer")

	// 是否允许序列化未导出字段
	c.AllowUnexportedParams = c.getBoolEnv("AllowUnexportedParams")
}

// getStringEnv 获取字符串环境变量
func (c *Config) getStringEnv(fieldName string) string {
	field := configFields[fieldName]
	envValue := os.Getenv(field.envKey)

	// 如果环境变量为空，返回默认值
	if envValue == "" {
		return field.defaultValue.(string)
	}

	// 如果有验证器，验证值的有效性
	if field.validator != nil && !field.validator(envValue) {
		return field.defaultValue.(string)
	}

	return envValue
}

// getIntEnv 获取整数环境变量
func (c *Config) getIntEnv(fieldName string) int {
	field := configFields[fieldName]
	envValue := os.Getenv(field.envKey)

	// 如果环境变量为空，返回默认值
	if envValue == "" {
		return field.defaultValue.(int)
	}

	// 尝试转换为整数
	if intValue, err := strconv.Atoi(envValue); err == nil {
		// 如果有验证器，验证值的有效性
		if field.validator == nil || field.validator(envValue) {
			return intValue
		}
	}

	// 转换失败或验证失败，返回默认值
	return field.defaultValue.(int)
}

// getUint64Env 获取无符号整数环境变量
func (c *Config) getUint64Env(fieldName string) uint64 {
	field := configFields[fieldName]
	envValue := os.Getenv(field.envKey)

	// 如果环境变量为空，返回默认值
	if envValue == "" {
		return field.defaultValue.(uint64)
	}

	// 尝试转换为无符号整数
	if uint64Value, err := strconv.ParseUint(envValue, 10, 64); err == nil {
		// 如果有验证器，验证值的有效性
		if field.validator == nil || field.validator(envValue) {
			return uint64Value
		}
	}

	// 转换失败或验证失败，返回默认值
	return field.defaultValue.(uint64)
}

// getBoolEnv 获取布尔值环境变量
func (c *Config) getBoolEnv(fieldName string) bool {
	field := configFields[fieldName]
	envValue := os.Getenv(field.envKey)

	// 如果环境变量为空，返回默认值
	if envValue == "" {
		return field.defaultValue.(bool)
	}

	// 尝试转换为布尔值
	if boolValue, err := strconv.ParseBool(envValue); err == nil {
		return boolValue
	}

	// 转换失败，返回默认值
	return field.defaultValue.(bool)
}

// getStringSliceEnv 获取字符串切片环境变量
func (c *Config) getStringSliceEnv(fieldName string) []string {
	field := configFields[fieldName]
	envValue := os.Getenv(field.envKey)

	// 如果环境变量为空，使用默认值
	if envValue == "" {
		return strings.Split(field.defaultValue.(string), ",")
	}

	return strings.Split(envValue, ",")
}

// CreateSpewConfig 根据配置创建spew配置
func (c *Config) CreateSpewConfig() *objDump.ConfigState {
	return &objDump.ConfigState{
		MaxDepth:                c.MaxDepth,
		SkipNilValues:           false,
		MaxElementsPerContainer: c.MaxElementsPerContainer,
		AllowUnexported:         c.AllowUnexportedParams,
		CompactLargeObjects:     true,
	}
}

// Validate 验证配置的有效性
func (c *Config) Validate() error {
	// 这里可以添加更复杂的配置验证逻辑
	return nil
}

// String 返回配置的字符串表示
func (c *Config) String() string {
	return "Config{" +
		"MonitorInterval: " + strconv.Itoa(c.MonitorInterval) + ", " +
		"MaxDepth: " + strconv.Itoa(c.MaxDepth) + ", " +
		"IgnoreNames: [" + strings.Join(c.IgnoreNames, ",") + "], " +
		"MemoryLimit: " + strconv.FormatUint(c.MemoryLimit, 10) + ", " +
		"MemoryCheckInterval: " + strconv.Itoa(c.MemoryCheckInterval) + ", " +
		"DBType: " + c.DBType + ", " +
		"InsertMode: " + c.InsertMode + ", " +
		"ParamStoreMode: " + c.ParamStoreMode + ", " +
		"LogFileName: " + c.LogFileName +
		"}"
}
