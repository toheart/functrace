package trace

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigCreation(t *testing.T) {
	// 测试默认配置
	config := NewConfig()

	assert.Equal(t, DefaultMonitorInterval, config.MonitorInterval)
	assert.Equal(t, DefaultMaxDepth, config.MaxDepth)
	assert.Equal(t, "sqlite", config.DBType)
	assert.Equal(t, SyncMode, config.InsertMode)
	assert.Equal(t, ParamStoreModeNone, config.ParamStoreMode)
	assert.NotEmpty(t, config.IgnoreNames)
}

func TestConfigWithEnvironmentVariables(t *testing.T) {
	// 设置环境变量
	envVars := map[string]string{
		EnvGoroutineMonitorInterval: "30",
		EnvMaxDepth:                 "5",
		EnvIgnoreNames:              "test,mock,fake",
		EnvDBType:                   "memory",
		EnvDBInsertMode:             "async",
		EnvParamStoreMode:           "all",
	}

	// 保存原始环境变量
	originalVars := make(map[string]string)
	for key, value := range envVars {
		originalVars[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	// 清理函数
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// 创建配置
	config := NewConfig()

	// 验证配置值
	assert.Equal(t, 30, config.MonitorInterval)
	assert.Equal(t, 5, config.MaxDepth)
	assert.Equal(t, []string{"test", "mock", "fake"}, config.IgnoreNames)
	assert.Equal(t, "memory", config.DBType)
	assert.Equal(t, AsyncMode, config.InsertMode)
	assert.Equal(t, ParamStoreModeAll, config.ParamStoreMode)
}

func TestConfigValidation(t *testing.T) {
	// 测试无效的环境变量值回退到默认值
	envVars := map[string]string{
		EnvGoroutineMonitorInterval: "invalid",
		EnvMaxDepth:                 "0",
		EnvDBInsertMode:             "invalid_mode",
		EnvParamStoreMode:           "invalid_mode",
	}

	// 保存原始环境变量
	originalVars := make(map[string]string)
	for key, value := range envVars {
		originalVars[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	// 清理函数
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// 创建配置
	config := NewConfig()

	// 验证回退到默认值
	assert.Equal(t, DefaultMonitorInterval, config.MonitorInterval)
	assert.Equal(t, DefaultMaxDepth, config.MaxDepth)
	assert.Equal(t, SyncMode, config.InsertMode)
	assert.Equal(t, ParamStoreModeNone, config.ParamStoreMode)
}

func TestCreateSpewConfig(t *testing.T) {
	config := &Config{MaxDepth: 5}
	spewConfig := config.CreateSpewConfig()

	assert.NotNil(t, spewConfig)
	assert.Equal(t, 6, spewConfig.MaxDepth) // MaxDepth + 1
	assert.True(t, spewConfig.SkipNilValues)
}

func TestConfigString(t *testing.T) {
	config := &Config{
		MonitorInterval: 10,
		MaxDepth:        3,
		IgnoreNames:     []string{"log", "test"},
		DBType:          "sqlite",
		InsertMode:      SyncMode,
		ParamStoreMode:  ParamStoreModeNone,
		LogFileName:     "./test.log",
	}

	configStr := config.String()
	assert.Contains(t, configStr, "MonitorInterval: 10")
	assert.Contains(t, configStr, "MaxDepth: 3")
	assert.Contains(t, configStr, "IgnoreNames: [log,test]")
	assert.Contains(t, configStr, "DBType: sqlite")
	assert.Contains(t, configStr, "InsertMode: sync")
	assert.Contains(t, configStr, "ParamStoreMode: none")
}
