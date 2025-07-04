package trace

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// MemoryMonitor 内存监控器
type MemoryMonitor struct {
	threshold     uint64        // 内存阈值（字节）
	checkInterval time.Duration // 检查间隔
	enabled       bool          // 是否启用监控
	stopChan      chan struct{} // 停止信号
	logger        *logrus.Logger
}

// NewMemoryMonitor 创建新的内存监控器
func NewMemoryMonitor(threshold uint64, checkInterval time.Duration, logger *logrus.Logger) *MemoryMonitor {
	return &MemoryMonitor{
		threshold:     threshold,
		checkInterval: checkInterval,
		enabled:       false,
		stopChan:      make(chan struct{}),
		logger:        logger,
	}
}

// Start 启动内存监控
func (m *MemoryMonitor) Start() {
	if m.enabled {
		return // 已经启动
	}

	m.enabled = true
	m.logger.WithFields(logrus.Fields{
		"threshold": humanReadableBytes(m.threshold),
		"interval":  m.checkInterval,
	}).Info("starting memory monitor for 'all' parameter store mode")

	go m.monitorLoop()
}

// Stop 停止内存监控
func (m *MemoryMonitor) Stop() {
	if !m.enabled {
		return // 已经停止
	}

	m.enabled = false
	close(m.stopChan)
	m.logger.Info("memory monitor stopped")
}

// monitorLoop 监控循环
func (m *MemoryMonitor) monitorLoop() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.checkMemoryUsage() {
				m.emergencyExit()
				return
			}
		case <-m.stopChan:
			m.logger.Debug("memory monitor loop stopped")
			return
		}
	}
}

// checkMemoryUsage 检查内存使用量
// 返回 true 表示超过阈值，需要紧急退出
func (m *MemoryMonitor) checkMemoryUsage() bool {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// 计算总内存使用量
	// 包括堆内存、栈内存、其他系统内存
	totalMemory := stats.Sys

	m.logger.WithFields(logrus.Fields{
		"current_memory": humanReadableBytes(totalMemory),
		"threshold":      humanReadableBytes(m.threshold),
		"heap_inuse":     humanReadableBytes(stats.HeapInuse),
		"stack_inuse":    humanReadableBytes(stats.StackInuse),
		"sys_memory":     humanReadableBytes(stats.Sys),
	}).Debug("memory usage check")

	if totalMemory > m.threshold {
		m.logger.WithFields(logrus.Fields{
			"current_memory": humanReadableBytes(totalMemory),
			"threshold":      humanReadableBytes(m.threshold),
			"exceeded_by":    humanReadableBytes(totalMemory - m.threshold),
		}).Error("memory threshold exceeded, initiating emergency exit")
		return true
	}

	return false
}

// emergencyExit 紧急退出程序
func (m *MemoryMonitor) emergencyExit() {
	// 获取当前内存使用情况
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// 立即输出错误信息到标准错误
	fmt.Fprintf(os.Stderr, "\n=== FUNCTRACE MEMORY PROTECTION ===\n")
	fmt.Fprintf(os.Stderr, "FATAL: Memory usage exceeded %s limit in 'all' parameter store mode.\n",
		humanReadableBytes(m.threshold))
	fmt.Fprintf(os.Stderr, "Current memory usage: %s\n", humanReadableBytes(stats.Sys))
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "SUGGESTION: Try following options to reduce memory usage:\n")
	fmt.Fprintf(os.Stderr, "1. Reduce trace depth:\n")
	fmt.Fprintf(os.Stderr, "  - Set: FUNCTRACE_MAX_DEPTH=2 (default is 3)\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "2. Use lighter parameter store mode:\n")
	fmt.Fprintf(os.Stderr, "  - Set: FUNCTRACE_PARAM_STORE_MODE=normal (moderate memory usage)\n")
	fmt.Fprintf(os.Stderr, "  - Set: FUNCTRACE_PARAM_STORE_MODE=none (minimal memory usage)\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Program will exit immediately to prevent system OOM.\n")
	fmt.Fprintf(os.Stderr, "===================================\n\n")

	// 强制刷新标准错误输出
	os.Stderr.Sync()

	// 记录最后的日志（如果可能）
	m.logger.WithFields(logrus.Fields{
		"current_memory": humanReadableBytes(stats.Sys),
		"threshold":      humanReadableBytes(m.threshold),
		"reason":         "memory_limit_exceeded",
	}).Fatal("emergency exit initiated due to memory limit")

	// 直接退出程序
	os.Exit(1)
}

// humanReadableBytes 将字节数转换为人类可读的格式
func humanReadableBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// IsEnabled 检查内存监控是否启用
func (m *MemoryMonitor) IsEnabled() bool {
	return m.enabled
}

// GetThreshold 获取内存阈值
func (m *MemoryMonitor) GetThreshold() uint64 {
	return m.threshold
}

// SetThreshold 设置内存阈值
func (m *MemoryMonitor) SetThreshold(threshold uint64) {
	m.threshold = threshold
	if m.enabled {
		m.logger.WithFields(logrus.Fields{
			"new_threshold": humanReadableBytes(threshold),
		}).Info("memory threshold updated")
	}
}
