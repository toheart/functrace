# Functrace Example Project

这个示例项目展示了如何使用 `functrace` 库进行函数跟踪和性能分析。

## 项目结构

```
example/
├── go.mod          # Go模块文件
├── go.sum          # 依赖校验文件
├── main.go         # 主程序文件
└── README.md       # 说明文档
```

## 功能特性

这个示例程序包含了以下测试场景：

### 1. 基本功能测试 (Basic Function Tests)
- 简单参数跟踪
- 字符串参数处理
- 数字参数处理
- 空参数处理

### 2. 结构体测试 (Struct Tests)
- 值接收者方法测试
- 复杂结构体序列化
- 嵌套结构体处理

### 3. 指针测试 (Pointer Tests)
- 指针接收者方法测试
- 指针参数差异计算
- 对象状态变化跟踪

### 4. 接收者函数测试 (Receiver Function Tests)
- 值接收者方法测试
- 方法调用参数跟踪
- 接收者状态不变验证

### 5. 接收者指针函数测试 (Receiver Pointer Function Tests)
- 指针接收者方法测试
- 接收者状态变更跟踪
- 对象生命周期管理
- 数据增删改查操作

### 6. Goroutine测试 (Goroutine Tests)
- 并发函数调用跟踪
- 多goroutine场景
- 线程安全验证

### 7. 复杂场景测试 (Complex Scenario Tests)
- 嵌套函数调用
- 多层调用栈跟踪
- 复杂调用关系分析

### 8. 性能测试 (Performance Tests)
- 大量函数调用
- 性能开销评估
- 内存使用监控

### 9. 错误处理测试 (Error Handling Tests)
- Panic恢复机制
- 异常情况处理
- 错误边界测试

### 10. 内存使用测试 (Memory Usage Tests)
- 大数据对象处理
- 内存占用监控
- 垃圾回收影响

## 接收者函数测试详解

### 值接收者方法 (Value Receiver Methods)
```go
// AddUser 添加用户（值接收者方法）
func (us UserService) AddUser(user *User) {
    defer functrace.Trace([]interface{}{"AddUser", user})()
    // 方法实现
}
```

**特点：**
- 接收者是值的副本
- 方法内对接收者的修改不会影响原对象
- 适用于只读操作或不需要修改接收者状态的场景

### 指针接收者方法 (Pointer Receiver Methods)
```go
// CreateUser 创建用户（指针接收者方法）
func (us *UserService) CreateUser(name string, age int) *User {
    defer functrace.Trace([]interface{}{"CreateUser", name, age})()
    // 修改接收者状态
    us.users[user.ID] = user
    us.count++
    return user
}
```

**特点：**
- 接收者是指针
- 方法内对接收者的修改会影响原对象
- 适用于需要修改接收者状态的场景

### 状态变更测试场景

1. **创建操作**：`CreateUser` 方法创建新用户并更新服务状态
2. **更新操作**：`UpdateUser` 方法修改现有用户信息
3. **删除操作**：`DeleteUser` 方法删除用户并更新计数
4. **查询操作**：`GetUser` 和 `GetStats` 方法进行只读操作

## 运行方法

### 1. 进入示例目录
```bash
cd example
```

### 2. 初始化依赖
```bash
go mod tidy
```

### 3. 运行示例程序
```bash
go run main.go
```

### 4. 查看输出结果
程序会输出详细的跟踪信息，包括：
- 函数调用栈
- 参数序列化结果
- 执行时间统计
- 内存使用情况
- 接收者状态变更

## 配置说明

### 环境变量设置
```go
os.Setenv("FUNCTRACE_PARAM_STORE_MODE", "all")
```

### 日志级别设置
```go
logger := functrace.GetLogger()
logger.SetLevel(0) // Debug级别，显示详细信息
```

### 跟踪参数
```go
defer functrace.Trace([]interface{}{"参数1", "参数2"})()
```

## 预期输出

运行程序后，您将看到类似以下的输出：

```
=== Functrace Example Program ===

--- Receiver Function Tests ---
UserService.AddUser: Added user Alice, count: 1
Initial stats: map[total_users:0 user_ids:0]

--- Receiver Pointer Function Tests ---
UserService.CreateUser: Created user Bob with ID 1, total count: 1
Created user: &{ID:1 Name:Bob Age:30}
UserService.CreateUser: Created user Charlie with ID 2, total count: 2
Created user: &{ID:2 Name:Charlie Age:35}
UserService.UpdateUser: Updated user ID 1 to Bob Updated, age 31
Update result: true
Updated user: &{ID:1 Name:Bob Updated Age:31}
UserService.DeleteUser: Deleted user ID 2, remaining count: 1
Delete result: true
Deleted user: <nil>
Final stats: map[total_users:1 user_ids:1]
```

## 注意事项

1. **性能影响**: 跟踪功能会带来一定的性能开销，建议在生产环境中谨慎使用
2. **内存使用**: 大量函数调用可能会占用较多内存，注意监控内存使用情况
3. **并发安全**: 示例程序包含了并发测试，验证了库的线程安全性
4. **错误处理**: 程序包含了panic恢复机制，确保异常情况下能够正常退出
5. **接收者状态**: 指针接收者方法会修改对象状态，注意状态一致性

## 扩展测试

您可以根据需要添加更多测试场景：

- 网络请求跟踪
- 数据库操作跟踪
- 文件I/O操作跟踪
- 第三方库集成测试
- 自定义配置测试
- 更多接收者方法测试

## 故障排除

如果遇到问题，请检查：

1. Go版本是否兼容（需要Go 1.20+）
2. 依赖是否正确安装
3. 权限是否足够（数据库文件写入）
4. 内存是否充足（大数据测试）
5. 环境变量是否正确设置

## 贡献

欢迎提交Issue和Pull Request来改进这个示例项目！ 