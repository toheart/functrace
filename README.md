# FuncTrace 函数跟踪库

FuncTrace 是一个用于跟踪和分析 Go 函数调用的工具库。本项目采用领域驱动设计 (DDD) 架构，将业务逻辑与基础设施分离，提高了代码的可测试性和可维护性。

## 项目架构

项目采用分层架构，主要包括以下几个核心模块：

### 领域层 (Domain)

领域层包含业务核心概念和规则，独立于基础设施和应用层：

- `domain/model/`: 包含领域实体和值对象，例如 `TraceData`、`ParamStoreData` 和 `GoroutineTrace`
- `domain/repository.go`: 定义仓储接口，用于数据持久化操作

### 应用层 (App)

应用层协调领域对象和仓储，实现业务用例：

- `app/app.go`: 提供应用服务，协调仓储和领域对象的交互

### 基础设施层 (Infrastructure)

基础设施层提供领域模型的实现和技术服务：

- `persistence/`: 持久化相关实现
  - `factory/`: 包含仓储工厂和数据库接口定义
    - `factory.go`: 统一管理数据库实例和仓储工厂的创建、初始化和资源释放
  - `sqlite/`: SQLite 实现
  - `mock/`: 测试用的模拟实现

## 核心流程

1. 创建应用实例 (`app.NewTraceApp`)
2. 获取仓储 (`app.GetTraceRepository()`)
3. 使用领域模型和仓储进行业务操作
4. 关闭应用 (`app.Close()`)，工厂会自动释放资源

## 使用示例

参考 `examples/usage_example.go` 了解如何使用新的架构：

```go
// 初始化应用
traceApp, err := app.NewTraceApp(logger)
if err != nil {
    logger.WithError(err).Fatal("创建应用程序实例失败")
    return
}
defer traceApp.Close()

// 获取仓储
traceRepo := traceApp.GetTraceRepository()

// 创建并保存跟踪数据
traceData := model.NewTraceData(...)
traceID, err := traceRepo.SaveTrace(traceData)
```

## 设计优势

1. **关注点分离**：领域逻辑与技术细节分离
2. **可测试性**：仓储接口可以轻松实现模拟版本
3. **可扩展性**：容易添加新的存储类型和业务功能
4. **维护性**：代码结构清晰，责任边界明确
5. **资源管理**：工厂模式负责资源生命周期，简化应用程序
6. **结构优化**：将工厂和数据库接口整合到persistence下，更符合DDD分层原则

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