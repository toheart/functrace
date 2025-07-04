package trace

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestMemoryMonitor 测试内存监控器基本功能
func TestMemoryMonitor(t *testing.T) {
	// 创建测试用的logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// 创建一个内存阈值很小的监控器用于测试（1MB）
	threshold := uint64(1 * 1024 * 1024)
	monitor := NewMemoryMonitor(threshold, 1*time.Second, logger)

	// 测试初始状态
	if monitor.IsEnabled() {
		t.Error("memory monitor should not be enabled initially")
	}

	if monitor.GetThreshold() != threshold {
		t.Errorf("expected threshold %d, got %d", threshold, monitor.GetThreshold())
	}

	// 测试设置阈值
	newThreshold := uint64(2 * 1024 * 1024)
	monitor.SetThreshold(newThreshold)
	if monitor.GetThreshold() != newThreshold {
		t.Errorf("expected threshold %d, got %d", newThreshold, monitor.GetThreshold())
	}

	// 测试启动和停止
	monitor.Start()
	if !monitor.IsEnabled() {
		t.Error("memory monitor should be enabled after start")
	}

	// 等待一段时间确保监控器运行
	time.Sleep(100 * time.Millisecond)

	monitor.Stop()
	if monitor.IsEnabled() {
		t.Error("memory monitor should not be enabled after stop")
	}
}

// TestMemoryMonitorConfig 测试配置相关功能
func TestMemoryMonitorConfig(t *testing.T) {
	// 创建配置
	config := NewConfig()

	// 验证默认内存配置
	if config.MemoryLimit != DefaultMemoryLimit {
		t.Errorf("expected default memory limit %d, got %d", DefaultMemoryLimit, config.MemoryLimit)
	}

	if config.MemoryCheckInterval != DefaultMemoryCheckInterval {
		t.Errorf("expected default memory check interval %d, got %d", DefaultMemoryCheckInterval, config.MemoryCheckInterval)
	}
}

// TestTraceInstanceMemoryMonitor 测试TraceInstance中的内存监控集成
func TestTraceInstanceMemoryMonitor(t *testing.T) {
	// 创建配置
	config := NewConfig()

	// 验证默认内存配置
	if config.MemoryLimit != DefaultMemoryLimit {
		t.Errorf("expected default memory limit %d, got %d", DefaultMemoryLimit, config.MemoryLimit)
	}

	if config.MemoryCheckInterval != DefaultMemoryCheckInterval {
		t.Errorf("expected default memory check interval %d, got %d", DefaultMemoryCheckInterval, config.MemoryCheckInterval)
	}

	// 测试内存监控器创建
	logger := logrus.New()
	monitor := NewMemoryMonitor(config.MemoryLimit, time.Duration(config.MemoryCheckInterval)*time.Second, logger)

	if monitor == nil {
		t.Error("memory monitor should be created")
	}

	if monitor.GetThreshold() != config.MemoryLimit {
		t.Errorf("expected threshold %d, got %d", config.MemoryLimit, monitor.GetThreshold())
	}
}

// TestHumanReadableBytes 测试字节格式化功能
func TestHumanReadableBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{2147483648, "2.00 GB"},
	}

	for _, test := range tests {
		result := humanReadableBytes(test.bytes)
		if result != test.expected {
			t.Errorf("for %d bytes, expected %s, got %s", test.bytes, test.expected, result)
		}
	}
}
