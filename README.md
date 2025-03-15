# 函数跟踪工具

## 概述

该工具用于跟踪 Go 语言中的函数调用，记录函数的执行时间和相关信息。它支持对 goroutine 的监控，能够记录每个 goroutine 的 CPU 执行时间和总运行时间。

## 功能

- **函数跟踪**：使用装饰器模式跟踪函数的进入和退出，记录参数、执行时间等信息。
- **goroutine 监控**：定期检查正在运行的 goroutine，记录其执行时间和状态。
- **数据库支持**：将跟踪数据存储在 SQLite 数据库中，支持异步插入和更新操作。

## 数据库结构

### TraceData 表

- `id`: 唯一标识符
- `name`: 函数名称
- `gid`: goroutine ID
- `indent`: 缩进级别
- `params`: 参数 JSON 字符串
- `timeCost`: CPU 执行时间
- `parentId`: 父函数 ID
- `createdAt`: 创建时间
- `seq`: 序列号

### GoroutineTrace 表

- `id`: 自增 ID
- `gid`: goroutine ID
- `timeCost`: CPU 执行时间
- `execTime`: 总运行时间
- `createTime`: 创建时间
- `isFinished`: 是否完成
- `initFuncName`: 初始函数名称

## 环境变量

- `TRACE_CHANNEL_COUNT`: 设置异步数据库操作的通道数量，默认为 10。
- `IGNORE_NAMES`: 定义默认忽略的函数名称关键字，多个名称用逗号分隔。
- `GOROUTINE_MONITOR_INTERVAL`: 设置监控 goroutine 运行时间的间隔，单位为秒，默认为 60 秒。

## 使用方法

1. 初始化跟踪实例：
   ```go
   instance := NewTraceInstance()
   ```

2. 使用 `Trace` 装饰器跟踪函数：
   ```go
   func MyFunction() {
       defer Trace(nil)() // 记录函数调用
       // 函数逻辑
   }
   ```

3. 监控 goroutine：
   - 监控功能会自动启动，定期检查 goroutine 的状态并更新数据库。

## 注意事项

- 确保在使用前正确配置数据库连接和环境变量。
- 监控功能会在后台运行，可能会影响性能，建议在生产环境中谨慎使用。

## 贡献

欢迎提交问题和贡献代码！请遵循贡献指南。