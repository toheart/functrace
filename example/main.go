package main

import (
	"fmt"
	"log"
	"time"

	"github.com/toheart/functrace"
)

// User 用户结构体
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Order 订单结构体
type Order struct {
	ID     int     `json:"id"`
	UserID int     `json:"user_id"`
	Amount float64 `json:"amount"`
	Items  []Item  `json:"items"`
}

// Item 商品结构体
type Item struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// UserService 用户服务结构体
type UserService struct {
	users map[int]*User
	count int
}

// NewUserService 创建新的用户服务
func NewUserService() *UserService {
	return &UserService{
		users: make(map[int]*User),
		count: 0,
	}
}

// AddUser 添加用户（值接收者方法）
func (us UserService) AddUser(user *User) {
	defer functrace.Trace([]interface{}{us, "AddUser", user})()
	time.Sleep(5 * time.Millisecond)

	// 模拟处理逻辑
	us.count++ // 这个修改不会影响原对象
	fmt.Printf("UserService.AddUser: Added user %s, count: %d\n", user.Name, us.count)
}

// GetUser 获取用户（值接收者方法）
func (us UserService) GetUser(id int) *User {
	defer functrace.Trace([]interface{}{us, "GetUser", id})()
	time.Sleep(3 * time.Millisecond)

	user, exists := us.users[id]
	if !exists {
		return nil
	}
	return user
}

// CreateUser 创建用户（指针接收者方法）
func (us *UserService) CreateUser(name string, age int) *User {
	defer functrace.Trace([]interface{}{us, "CreateUser", name, age})()
	time.Sleep(5 * time.Millisecond)

	// 创建新用户
	user := &User{
		ID:   us.count + 1,
		Name: name,
		Age:  age,
	}

	// 修改接收者状态
	us.users[user.ID] = user
	us.count++

	fmt.Printf("UserService.CreateUser: Created user %s with ID %d, total count: %d\n", name, user.ID, us.count)
	return user
}

// UpdateUser 更新用户（指针接收者方法）
func (us *UserService) UpdateUser(id int, name string, age int) bool {
	defer functrace.Trace([]interface{}{us, "UpdateUser", id, name, age})()
	time.Sleep(4 * time.Millisecond)

	user, exists := us.users[id]
	if !exists {
		return false
	}

	// 修改接收者中的用户数据
	user.Name = name
	user.Age = age

	fmt.Printf("UserService.UpdateUser: Updated user ID %d to %s, age %d\n", id, name, age)
	return true
}

// DeleteUser 删除用户（指针接收者方法）
func (us *UserService) DeleteUser(id int) bool {
	defer functrace.Trace([]interface{}{us, "DeleteUser", id})()
	time.Sleep(3 * time.Millisecond)

	_, exists := us.users[id]
	if !exists {
		return false
	}

	// 修改接收者状态
	delete(us.users, id)
	us.count--

	fmt.Printf("UserService.DeleteUser: Deleted user ID %d, remaining count: %d\n", id, us.count)
	return true
}

// GetStats 获取统计信息（值接收者方法）
func (us UserService) GetStats() map[string]interface{} {
	defer functrace.Trace([]interface{}{us, "GetStats"})()
	time.Sleep(2 * time.Millisecond)

	return map[string]interface{}{
		"total_users": us.count,
		"user_ids":    len(us.users),
	}
}

func main() {
	fmt.Println("=== Functrace Example Program ===")

	// 初始化跟踪（现在默认使用all模式）
	defer functrace.Trace([]interface{}{"Main Program"})()

	// 设置日志级别
	logger := functrace.GetLogger()
	logger.SetLevel(0) // 设置为Debug级别以查看详细信息

	// 运行各种测试场景
	runBasicTests()
	runStructTests()
	runPointerTests()
	runReceiverTests()
	runReceiverPointerTests()
	runGoroutineTests()
	runComplexTests()
	runPerformanceTests()
	runErrorTests()
	runMemoryTests()

	// 等待一段时间让异步操作完成
	time.Sleep(100 * time.Millisecond)

	// 关闭跟踪实例
	if err := functrace.CloseTraceInstance(); err != nil {
		log.Printf("Failed to close trace instance: %v", err)
	}

	fmt.Println("=== Example program execution completed ===")
}

// runBasicTests 基本功能测试
func runBasicTests() {
	fmt.Println("\n--- Basic Function Tests ---")

	// 测试简单函数
	defer functrace.Trace([]interface{}{"Basic Test", 123, true})()
	time.Sleep(10 * time.Millisecond)

	// 测试字符串参数
	defer functrace.Trace([]interface{}{"Hello", "World", "Test"})()
	time.Sleep(5 * time.Millisecond)

	// 测试数字参数
	defer functrace.Trace([]interface{}{1, 2.5, 3.14, -10})()
	time.Sleep(5 * time.Millisecond)

	// 测试空参数
	defer functrace.Trace([]interface{}{})()
	time.Sleep(2 * time.Millisecond)
}

// runStructTests 结构体测试
func runStructTests() {
	fmt.Println("\n--- Struct Tests ---")

	user := User{
		ID:   1,
		Name: "John Doe",
		Age:  25,
	}

	order := Order{
		ID:     1001,
		UserID: 1,
		Amount: 299.99,
		Items: []Item{
			{ID: 1, Name: "Product 1", Price: 99.99, Quantity: 2},
			{ID: 2, Name: "Product 2", Price: 100.01, Quantity: 1},
		},
	}

	// 测试值接收者方法
	defer functrace.Trace([]interface{}{user, order})()
	processUser(user)
	time.Sleep(10 * time.Millisecond)
}

// runPointerTests 指针测试
func runPointerTests() {
	fmt.Println("\n--- Pointer Tests ---")

	user := &User{
		ID:   2,
		Name: "Jane Smith",
		Age:  30,
	}

	order := &Order{
		ID:     1002,
		UserID: 2,
		Amount: 599.99,
		Items: []Item{
			{ID: 3, Name: "Product 3", Price: 199.99, Quantity: 3},
		},
	}

	// 测试指针接收者方法
	defer functrace.Trace([]interface{}{user, order})()
	processUserPointer(user)
	time.Sleep(10 * time.Millisecond)
}

// runReceiverTests 接收者函数测试
func runReceiverTests() {
	fmt.Println("\n--- Receiver Function Tests ---")

	// 创建用户服务实例
	userService := NewUserService()

	// 测试值接收者方法
	defer functrace.Trace([]interface{}{"Receiver Tests Start"})()

	// 测试AddUser（值接收者）
	user1 := &User{ID: 1, Name: "Alice", Age: 25}
	userService.AddUser(user1)

	// 测试GetUser（值接收者）
	userService.GetUser(1)

	// 测试GetStats（值接收者）
	stats := userService.GetStats()
	fmt.Printf("Initial stats: %+v\n", stats)

	time.Sleep(10 * time.Millisecond)
}

// runReceiverPointerTests 接收者指针函数测试
func runReceiverPointerTests() {
	fmt.Println("\n--- Receiver Pointer Function Tests ---")

	// 创建用户服务实例
	userService := NewUserService()

	defer functrace.Trace([]interface{}{"Receiver Pointer Tests Start"})()

	// 测试CreateUser（指针接收者）- 创建用户并修改接收者状态
	user1 := userService.CreateUser("Bob", 30)
	fmt.Printf("Created user: %+v\n", user1)

	// 测试CreateUser（指针接收者）- 再次创建用户
	user2 := userService.CreateUser("Charlie", 35)
	fmt.Printf("Created user: %+v\n", user2)

	// 测试UpdateUser（指针接收者）- 更新用户信息
	success := userService.UpdateUser(1, "Bob Updated", 31)
	fmt.Printf("Update result: %t\n", success)

	// 验证更新后的状态
	updatedUser := userService.GetUser(1)
	fmt.Printf("Updated user: %+v\n", updatedUser)

	// 测试DeleteUser（指针接收者）- 删除用户
	deleteSuccess := userService.DeleteUser(2)
	fmt.Printf("Delete result: %t\n", deleteSuccess)

	// 验证删除后的状态
	deletedUser := userService.GetUser(2)
	fmt.Printf("Deleted user: %+v\n", deletedUser)

	// 获取最终统计信息
	finalStats := userService.GetStats()
	fmt.Printf("Final stats: %+v\n", finalStats)

	time.Sleep(15 * time.Millisecond)
}

// runGoroutineTests Goroutine测试
func runGoroutineTests() {
	fmt.Println("\n--- Goroutine Tests ---")

	// 测试并发场景
	for i := 0; i < 3; i++ {
		go func(id int) {
			defer functrace.Trace([]interface{}{"Goroutine", id})()
			time.Sleep(20 * time.Millisecond)
		}(i)
	}

	// 等待所有goroutine完成
	time.Sleep(100 * time.Millisecond)
}

// runComplexTests 复杂场景测试
func runComplexTests() {
	fmt.Println("\n--- Complex Scenario Tests ---")

	// 测试嵌套调用
	defer functrace.Trace([]interface{}{"Complex Test Start"})()
	complexFunction()
	time.Sleep(15 * time.Millisecond)
}

// runPerformanceTests 性能测试
func runPerformanceTests() {
	fmt.Println("\n--- Performance Tests ---")

	// 测试大量函数调用
	for i := 0; i < 5; i++ {
		defer functrace.Trace([]interface{}{"Performance Test", i})()
		time.Sleep(1 * time.Millisecond)
	}
}

// runErrorTests 错误处理测试
func runErrorTests() {
	fmt.Println("\n--- Error Handling Tests ---")

	// 测试panic恢复
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic: %v\n", r)
		}
	}()

	defer functrace.Trace([]interface{}{"Error Test"})()
	panicFunction()
}

// runMemoryTests 内存使用测试
func runMemoryTests() {
	fmt.Println("\n--- Memory Usage Tests ---")

	// 测试大对象
	largeData := make([]int, 1000)
	for i := range largeData {
		largeData[i] = i
	}

	defer functrace.Trace([]interface{}{"Large Data Test", len(largeData)})()
	processLargeData(largeData)
	time.Sleep(5 * time.Millisecond)
}

// processUser 处理用户信息（值接收者）
func processUser(user User) {
	defer functrace.Trace([]interface{}{user})()
	time.Sleep(5 * time.Millisecond)

	// 模拟处理逻辑
	user.Age++ // 修改不会影响原对象
}

// processUserPointer 处理用户信息（指针接收者）
func processUserPointer(user *User) {
	defer functrace.Trace([]interface{}{user})()
	time.Sleep(5 * time.Millisecond)

	// 模拟处理逻辑
	user.Age++ // 修改会影响原对象
}

// complexFunction 复杂函数，包含嵌套调用
func complexFunction() {
	defer functrace.Trace([]interface{}{"Complex Function"})()

	// 嵌套调用1
	func() {
		defer functrace.Trace([]interface{}{"Nested Function 1"})()
		time.Sleep(5 * time.Millisecond)

		// 嵌套调用2
		func() {
			defer functrace.Trace([]interface{}{"Nested Function 2"})()
			time.Sleep(3 * time.Millisecond)
		}()
	}()
}

// panicFunction 会panic的函数
func panicFunction() {
	defer functrace.Trace([]interface{}{"Panic Function"})()
	time.Sleep(2 * time.Millisecond)
	panic("Intentional panic for testing")
}

// processLargeData 处理大数据
func processLargeData(data []int) {
	defer functrace.Trace([]interface{}{"Process Large Data", len(data)})()
	time.Sleep(3 * time.Millisecond)

	// 模拟数据处理
	sum := 0
	for _, v := range data {
		sum += v
	}
}
