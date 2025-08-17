package objectdump

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMapDumperWithProtectedMap 测试使用保护的map
func TestMapDumperWithProtectedMap(t *testing.T) {
	// 创建一个有保护的map结构
	type SafeMap struct {
		mu   sync.RWMutex
		data map[string]int
	}

	safeMap := &SafeMap{
		data: make(map[string]int),
	}

	// 初始化一些数据
	for i := 0; i < 50; i++ {
		safeMap.mu.Lock()
		safeMap.data[fmt.Sprintf("key_%d", i)] = i
		safeMap.mu.Unlock()
	}

	var wg sync.WaitGroup

	// 启动读取goroutine
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				safeMap.mu.RLock()
				result := ToJSON(safeMap.data)
				safeMap.mu.RUnlock()

				if len(result) == 0 {
					t.Errorf("Got empty result")
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// 启动写入goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			safeMap.mu.Lock()
			safeMap.data[fmt.Sprintf("new_key_%d", j)] = j + 1000
			safeMap.mu.Unlock()
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()
	t.Log("Protected map test completed successfully")
}

// TestFastMapCopy 测试快速拷贝机制
func TestFastMapCopy(t *testing.T) {
	// 创建一个大map
	bigMap := make(map[string]int)
	for i := 0; i < 1000; i++ {
		bigMap[fmt.Sprintf("key_%d", i)] = i
	}

	// 测试默认限制
	result := ToJSON(bigMap)
	t.Logf("大map结果长度: %d bytes", len(result))

	// 验证结果包含截断信息
	if !strings.Contains(result, "truncated") {
		t.Log("注意：大map可能没有被截断")
	}

	// 测试自定义限制
	config := &ConfigState{
		MaxElementsPerContainer: 5, // 只取5个元素
		SkipNilValues:           false,
	}

	limitedResult := config.ToJSON(bigMap)
	t.Logf("限制5个元素的结果: %s", limitedResult)

	// 验证限制生效
	if !strings.Contains(limitedResult, "truncated") {
		t.Error("期望看到截断信息")
	}
}

// BenchmarkFastMapCopy 基准测试快速拷贝性能
func BenchmarkFastMapCopy(b *testing.B) {
	// 创建测试map
	testMap := make(map[string]int)
	for i := 0; i < 100; i++ {
		testMap[fmt.Sprintf("key_%d", i)] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToJSON(testMap)
	}
}

// TestMapDumperBasicSafety 基础安全测试
func TestMapDumperBasicSafety(t *testing.T) {
	// 测试各种map类型
	testCases := []interface{}{
		map[string]int{"a": 1, "b": 2},
		map[int]string{1: "one", 2: "two"},
		map[string]interface{}{
			"string": "value",
			"number": 42,
			"nested": map[string]int{"x": 1, "y": 2},
		},
		make(map[string]int),  // 空map
		(map[string]int)(nil), // nil map
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			result := ToJSON(testCase)
			if len(result) == 0 {
				t.Errorf("Case %d: got empty result", i)
			}
			t.Logf("Case %d result: %s", i, result)
		})
	}
}
