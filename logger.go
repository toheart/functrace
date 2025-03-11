package functrace

import (
	"fmt"
	"strings"
	"time"
)

// logFunctionEntry 记录函数进入的日志
func (t *TraceInstance) logFunctionEntry(id uint64, name string, indent int, parentId int64, paramsJSON string, startTime time.Time) {
	// 生成缩进字符串
	indents := generateIndentString(indent)

	// 构建日志输出
	var logBuilder strings.Builder
	logBuilder.WriteString(fmt.Sprintf("%s -> %s", indents, name))

	if parentId != 0 {
		logBuilder.WriteString(fmt.Sprintf(" (parentId: %d)", parentId))
	}

	// 记录日志
	t.log.Info(logBuilder.String(),
		"goroutine", id,
		"params", paramsJSON,
		"time", startTime.Format(TimeFormatWithMillis))
}

// logFunctionExit 记录函数退出的日志
func (t *TraceInstance) logFunctionExit(id uint64, name string, indent int, durationStr string) {
	// 生成缩进字符串
	indents := generateIndentString(indent - 1)

	// 构建日志输出
	var logBuilder strings.Builder
	logBuilder.WriteString(fmt.Sprintf("%s <- %s", indents, name))

	// 记录日志
	t.log.Info(logBuilder.String(),
		"goroutine", id,
		"duration", durationStr,
		"time", time.Now().Format(TimeFormatWithMillis))
}

// generateIndentString 生成缩进字符串
func generateIndentString(indent int) string {
	return strings.Repeat(IndentFormat, indent)
}
