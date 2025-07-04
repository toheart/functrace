# FuncTrace - Go 函数追踪与性能分析库

[![Go 版本](https://img.shields.io/badge/Go-%3E%3D1.19-blue)](https://golang.org/)
[![许可证](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

FuncTrace 是一个专为 Go 应用程序设计的综合性函数追踪库。采用领域驱动设计 (DDD) 架构构建，提供详细的函数执行模式洞察、性能指标和 goroutine 生命周期分析。

**[English Documentation / 英文文档](README.md)**

## 功能特性

### 🔍 函数调用追踪
- **装饰器模式**：通过简单装饰器语法自动追踪函数进入/退出
- **调用链分析**：嵌套函数调用的完整父子关系映射
- **执行时序**：精确测量每个函数的 CPU 执行时间
- **层级显示**：基于调用深度的自动缩进，清晰可视化

### 📊 参数存储系统
三种灵活的参数存储模式，平衡功能性和内存使用：

#### `none` 模式（默认 - 内存高效）
- 仅记录函数调用链和执行时间
- 最小内存占用，适合生产环境
- 最适合不需要详细调试的性能监控

#### `normal` 模式（平衡）
- 捕获常规函数和值接收器方法的参数
- 中等内存使用量，适合开发环境
- 在调试能力和资源消耗之间取得良好平衡

#### `all` 模式（完整调试）
- 记录所有参数，包括复杂对象变化
- 使用 JSON Patch 技术对指针接收者参数变更进行增量存储，大幅减少重复数据，提升大对象跟踪效率
- 最高内存使用量，适合详细问题分析
- 包含内置内存保护机制

### 🚀 Goroutine 监控
- **实时追踪**：监控 goroutine 的创建、执行和终止
- **生命周期管理**：自动记录 goroutine 总执行时间
- **后台清理**：定期后台任务清理已完成的 goroutine 追踪
- **状态同步**：线程安全的 goroutine 状态管理
- **main.main 退出数据安全**：main.main 退出时自动等待所有 trace 数据持久化，确保数据完整性

### 🛡️ 内存保护
- **内存监控器**：在 `all` 模式下自动监控内存使用量
- **阈值保护**：默认 2GB 内存限制，超出时紧急退出防止 OOM
- **智能警报**：清晰的错误信息和解决方案建议
- **可配置限制**：通过环境变量自定义内存阈值

### 🧠 智能参数序列化增强
增强的 spew 包特性：
- **JSON 输出**：复杂对象的结构化 JSON 格式
- **内存池优化**：对象池减少内存分配开销
- **类型安全**：安全处理所有 Go 数据类型，包括不安全操作
- **循环引用检测**：防止无限递归和栈溢出
- **高级类型支持**：现已支持 interface、指针、字节数组等类型
- **MaxDepth 截断增强**：截断信息输出更详细（如 `__truncated__`、`num_fields`、`length`、`type`），便于调试

### 💾 数据持久化
支持多种存储后端的仓储模式：

#### SQLite 存储（默认）
- 三个主要表：`TraceData`、`GoroutineTrace`、`ParamStoreData`
- 支持同步和异步插入模式
- 自动创建索引优化查询性能
- WAL 模式改善并发访问

#### 内存存储
- 用于测试的 Mock 实现
- 高速内存操作
- 非常适合单元测试和开发

## 安装

```bash
go get github.com/toheart/functrace
```

## 快速开始

### 基础用法

```go
package main

import (
    "time"
    "github.com/toheart/functrace"
)

func ExampleFunction(name string, count int) {
    defer functrace.Trace([]interface{}{name, count})()
    
    // 你的函数逻辑
    for i := 0; i < count; i++ {
        processItem(name, i)
    }
}

func processItem(name string, index int) {
    defer functrace.Trace([]interface{}{name, index})()
    
    // 处理逻辑
    time.Sleep(10 * time.Millisecond)
}

func main() {
    defer functrace.CloseTraceInstance()
    
    ExampleFunction("test", 3)
}
```

### 高级配置

```go
package main

import (
    "os"
    "github.com/toheart/functrace"
)

func main() {
    // 配置参数存储模式
    os.Setenv("FUNCTRACE_PARAM_STORE_MODE", "normal")
    
    // 配置异步数据库操作
    os.Setenv("ENV_DB_INSERT_MODE", "async")
    
    // 配置内存限制 (2GB)
    os.Setenv("FUNCTRACE_MEMORY_LIMIT", "2147483648")
    
    defer functrace.CloseTraceInstance()
    
    // 你的应用程序逻辑
    YourApplicationLogic()
}
```

## 配置选项

FuncTrace 支持通过环境变量进行配置：

| 环境变量 | 默认值 | 描述 |
|---------|--------|------|
| `FUNCTRACE_PARAM_STORE_MODE` | `none` | 参数存储模式：`none`/`normal`/`all` |
| `ENV_DB_INSERT_MODE` | `sync` | 数据库插入模式：`sync`/`async` |
| `FUNCTRACE_MEMORY_LIMIT` | `2147483648` | 内存限制（字节）（默认 2GB） |
| `FUNCTRACE_IGNORE_NAMES` | `log,context,string` | 要忽略的函数名关键字（逗号分隔） |
| `FUNCTRACE_GOROUTINE_MONITOR_INTERVAL` | `10` | Goroutine 监控间隔（秒） |
| `FUNCTRACE_MAX_DEPTH` | `3` | 最大追踪深度 |

## 参数存储模式对比

| 模式 | 内存使用 | 功能特性 | 使用场景 |
|------|----------|----------|----------|
| `none` | 最小 | 函数调用链 + 执行时间 | 测试环境监控 |
| `normal` | 中等 | 常规函数参数 + 值方法 | 开发环境调试 |
| `all` | 高 | 所有参数 + 指针接收器差异 | 详细问题分析 |

## 数据库架构

### TraceData 表
- `id`：唯一标识符
- `name`：函数名称
- `gid`：Goroutine ID
- `indent`：缩进级别
- `paramsCount`：参数数量
- `timeCost`：CPU 执行时间
- `parentId`：父函数 ID
- `createdAt`：创建时间戳
- `isFinished`：完成状态
- `seq`：序列号

### GoroutineTrace 表
- `id`：自增 ID
- `originGid`：原始 Goroutine ID
- `timeCost`：CPU 执行时间
- `createTime`：创建时间
- `isFinished`：完成状态
- `initFuncName`：初始函数名

### ParamStoreData 表
- `id`：唯一标识符
- `traceId`：关联的 TraceData ID
- `position`：参数位置
- `data`：参数 JSON 数据
- `isReceiver`：是否为接收器参数
- `baseId`：基础参数 ID（用于增量存储）

## 架构设计

FuncTrace 遵循清晰的分层架构：

```
API 层 (functrace.go)
    ↓
核心层 (trace 包)
    ↓
领域层 (domain 包)
    ↓
持久层 (persistence 包)
```

### 关键组件

- **API 层**：简单的外部接口 (`functrace.go`)
- **核心层**：主要追踪逻辑 (`trace/`)
- **领域层**：业务模型和仓储接口 (`domain/`)
- **持久层**：数据存储实现 (`persistence/`)

## 性能考虑

### 内存优化
- 对象池减少垃圾回收
- 可配置内存限制与自动保护
- 高效的 JSON 序列化与增量存储

### 数据库优化
- 高吞吐量场景的异步插入模式
- 适当的索引以实现快速查询
- SQLite 的连接池和 WAL 模式

### 并发安全
- 线程安全的 goroutine 状态管理
- 尽可能使用无锁原子操作
- 共享数据结构的适当同步

## 最佳实践

1. **生产环境使用**：使用 `none` 参数模式配合 `async` 数据库模式
2. **开发环境**：使用 `normal` 参数模式进行平衡调试
3. **深度调试**：使用 `all` 参数模式配合内存监控
4. **资源管理**：退出前始终调用 `functrace.CloseTraceInstance()`
5. **选择性追踪**：使用忽略模式排除频繁调用的函数

## 使用示例

### 不同参数存储模式的使用

#### 1. 不保存参数模式（默认，推荐生产环境）
```bash
# 设置环境变量
export FUNCTRACE_PARAM_STORE_MODE=none

# 或者不设置（默认为 none）
go run your_app.go
```

#### 2. 保存普通参数模式（平衡模式）
```bash
# 设置环境变量
export FUNCTRACE_PARAM_STORE_MODE=normal

go run your_app.go
```

#### 3. 全保存参数模式（完整调试模式）
```bash
# 设置环境变量
export FUNCTRACE_PARAM_STORE_MODE=all

go run your_app.go
```

### 结构体方法追踪示例

```go
type UserService struct {
    db *Database
}

func (s *UserService) CreateUser(name string, email string) error {
    defer functrace.Trace([]interface{}{name, email})()
    
    // 创建用户逻辑
    return s.db.Save(&User{Name: name, Email: email})
}

func (s *UserService) GetUser(id int) (*User, error) {
    defer functrace.Trace([]interface{}{id})()
    
    // 获取用户逻辑
    return s.db.FindByID(id)
}
```

## 故障排除

### 常见问题

1. **内存使用过高**：
   - 切换到 `normal` 或 `none` 模式
   - 调整内存限制阈值
   - 增加忽略函数列表

2. **性能影响**：
   - 使用异步数据库模式
   - 减少追踪深度
   - 选择性追踪重要函数

3. **数据库锁定**：
   - 确保调用 `CloseTraceInstance()`
   - 检查 SQLite WAL 模式配置
   - 避免同时运行多个实例

## 贡献

我们欢迎贡献！请查看我们的[贡献指南](CONTRIBUTING.md)了解详情。

## 许可证

本项目使用 MIT 许可证 - 请查看 [LICENSE](LICENSE) 文件了解详情。

## 支持

- **文档**：[Wiki](https://github.com/toheart/functrace/wiki)
- **问题报告**：[GitHub Issues](https://github.com/toheart/functrace/issues)
- **讨论**：[GitHub Discussions](https://github.com/toheart/functrace/discussions)

## 致谢

- 使用 [spew](https://github.com/davecgh/go-spew) 进行高级数据序列化
- 使用 [SQLite](https://sqlite.org/) 进行高效数据持久化
- 受到各种 Go 性能分析和追踪工具的启发

## 注意事项

- 确保在使用前正确配置数据库连接和环境变量
- 监控功能会在后台运行，可能会影响性能，建议在生产环境中谨慎使用
- 在 `all` 模式下注意内存使用量，特别是处理大量数据时

## 测试与覆盖率

核心功能均有单元测试，目标覆盖率 80%+，推荐使用 `go test -cover` 检查。 