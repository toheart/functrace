# FuncTrace

FuncTrace 是一个用于跟踪 Go 函数调用的工具，可以记录函数的进入和退出，以及执行时间和参数信息。

## 项目结构

项目文件结构如下：

- `constants.go`: 常量和配置定义
- `types.go`: 类型定义
- `instance.go`: 核心结构和初始化相关
- `database.go`: 数据库操作相关
- `logger.go`: 日志记录相关
- `trace.go`: 核心跟踪功能
- `format.go`: 参数格式化相关

## 使用方法

```go
package main

import (
    "github.com/yourusername/functrace"
)

func main() {
    // 在函数开始处调用 Trace，并在函数结束时执行返回的函数
    defer functrace.Trace([]interface{}{arg1, arg2})()
    
    // 函数的其他代码...
}
```

## 配置选项

可以通过环境变量配置 FuncTrace 的行为：

- `TRACE_CHANNEL_COUNT`: 设置数据库操作通道数量，默认为 5
- `IGNORE_NAMES`: 设置要忽略的函数名称关键字，用逗号分隔

## 数据库

FuncTrace 使用 SQLite 数据库存储跟踪信息，数据库文件默认命名为 `trace_N.db`，其中 N 是一个自增的数字。

## 日志

日志文件默认保存在 `./trace.log`。