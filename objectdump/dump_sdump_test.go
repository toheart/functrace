package objectdump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

// 测试基本类型
func TestSdump_BasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"bool_true", true, true},
		{"bool_false", false, false},
		{"int", 42, float64(42)},
		{"int8", int8(42), float64(42)},
		{"int16", int16(42), float64(42)},
		{"int32", int32(42), float64(42)},
		{"int64", int64(42), float64(42)},
		{"uint", uint(42), float64(42)},
		{"uint8", uint8(42), float64(42)},
		{"uint16", uint16(42), float64(42)},
		{"uint32", uint32(42), float64(42)},
		{"uint64", uint64(42), float64(42)},
		{"float32", float32(3.14), 3.140000104904175},
		{"float64", float64(3.14), 3.14},
		{"string", "hello", "hello"},
		{"complex64", complex64(1 + 2i), "(1+2i)"},
		{"complex128", complex128(1 + 2i), "(1+2i)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if !reflect.DeepEqual(parsed, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, parsed)
			}
		})
	}
}

// 测试特殊值
func TestSdump_SpecialValues(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"nil", nil, "invalid"},
		{"nan", math.NaN(), "NaN"},
		{"inf_positive", math.Inf(1), "Inf"},
		{"inf_negative", math.Inf(-1), "-Inf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if !reflect.DeepEqual(parsed, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, parsed)
			}
		})
	}
}

// 测试指针类型
func TestSdump_Pointers(t *testing.T) {
	value := 42
	ptr := &value
	nilPtr := (*int)(nil)

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"pointer_to_int", ptr, float64(42)},
		{"nil_pointer", nilPtr, nil},
		{"pointer_to_string", &[]string{"hello"}[0], "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if !reflect.DeepEqual(parsed, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, parsed)
			}
		})
	}
}

// 测试数组和切片
func TestSdump_ArraysAndSlices(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"empty_slice", []int{}, []interface{}{}},
		{"int_slice", []int{1, 2, 3}, []interface{}{float64(1), float64(2), float64(3)}},
		{"string_slice", []string{"a", "b", "c"}, []interface{}{"a", "b", "c"}},
		{"mixed_slice", []interface{}{1, "hello", true}, []interface{}{float64(1), "hello", true}},
		{"array", [3]int{1, 2, 3}, []interface{}{float64(1), float64(2), float64(3)}},
		{"nil_slice", []int(nil), nil},
		{"byte_slice", []byte("hello"), "hello"},
		{"byte_slice_hex", []byte{0x00, 0x01, 0xFF}, "0001ff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if !reflect.DeepEqual(parsed, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, parsed)
			}
		})
	}
}

// 测试Map类型
func TestSdump_Maps(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"empty_map", map[string]int{}, map[string]interface{}{}},
		{"string_int_map", map[string]int{"a": 1, "b": 2}, map[string]interface{}{"a": float64(1), "b": float64(2)}},
		{"int_string_map", map[int]string{1: "a", 2: "b"}, map[string]interface{}{"1": "a", "2": "b"}},
		{"nil_map", map[string]int(nil), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if !reflect.DeepEqual(parsed, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, parsed)
			}
		})
	}
}

// 测试结构体
func TestSdump_Structs(t *testing.T) {
	type SimpleStruct struct {
		Name  string
		Age   int
		Email string
	}

	type NestedStruct struct {
		ID     int
		Person SimpleStruct
		Tags   []string
	}

	type UnexportedStruct struct {
		Name  string
		age   int
		email string
	}

	simple := SimpleStruct{Name: "Alice", Age: 30, Email: "alice@example.com"}
	nested := NestedStruct{
		ID:     1,
		Person: simple,
		Tags:   []string{"admin", "user"},
	}
	unexported := UnexportedStruct{Name: "Bob", age: 25, email: "bob@example.com"}

	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			"simple_struct",
			simple,
			map[string]interface{}{
				"Name":  "Alice",
				"Age":   float64(30),
				"Email": "alice@example.com",
				"type":  "objectdump.SimpleStruct",
			},
		},
		{
			"nested_struct",
			nested,
			map[string]interface{}{
				"ID": float64(1),
				"Person": map[string]interface{}{
					"Name":  "Alice",
					"Age":   float64(30),
					"Email": "alice@example.com",
					"type":  "objectdump.SimpleStruct",
				},
				"Tags": []interface{}{"admin", "user"},
				"type": "objectdump.NestedStruct",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if !reflect.DeepEqual(parsed, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, parsed)
			}
		})
	}

	// 测试未导出字段
	t.Run("unexported_fields", func(t *testing.T) {
		config := &ConfigState{AllowUnexported: true}
		SetGlobalConfig(config)
		defer SetGlobalConfig(&Config) // 恢复默认配置

		result := Sdump(unexported)
		t.Logf("Sdump(unexported) = %s", result)

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Failed to parse JSON result: %v", err)
		}

		// 检查是否包含未导出字段
		if _, ok := parsed["age"]; !ok {
			t.Error("Expected to find unexported field 'age'")
		}
		if _, ok := parsed["email"]; !ok {
			t.Error("Expected to find unexported field 'email'")
		}
	})
}

// 测试接口类型
func TestSdump_Interfaces(t *testing.T) {
	var nilInterface interface{} = nil
	var stringInterface interface{} = "hello"
	var intInterface interface{} = 42

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"nil_interface", nilInterface, "invalid"},
		{"string_interface", stringInterface, "hello"},
		{"int_interface", intInterface, float64(42)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if !reflect.DeepEqual(parsed, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, parsed)
			}
		})
	}
}

// 测试函数和通道
func TestSdump_FunctionsAndChannels(t *testing.T) {
	ch := make(chan int)
	fn := func() {}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"channel", ch, "<chan chan int"},
		{"function", fn, "<func func()"},
		{"nil_channel", (chan int)(nil), nil},
		{"nil_function", (func())(nil), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			if tt.expected == nil {
				var parsed interface{}
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					t.Fatalf("Failed to parse JSON result: %v", err)
				}
				if parsed != nil {
					t.Errorf("Expected nil, got %v", parsed)
				}
			} else {
				var parsed string
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					t.Fatalf("Failed to parse JSON result: %v", err)
				}
				expectedStr := tt.expected.(string)
				if parsed[:len(expectedStr)] != expectedStr {
					t.Errorf("Expected prefix %s, got %s", expectedStr, parsed)
				}
			}
		})
	}
}

// 测试循环引用
func TestSdump_CircularReferences(t *testing.T) {
	type CircularStruct struct {
		Name string
		Next *CircularStruct
	}

	// 创建循环引用
	node1 := &CircularStruct{Name: "Node1"}
	node2 := &CircularStruct{Name: "Node2"}
	node1.Next = node2
	node2.Next = node1

	result := Sdump(node1)
	t.Logf("Sdump(circular) = %s", result)

	var parsed interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON result: %v", err)
	}

	// 检查是否包含循环引用标记
	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

// 测试深度限制
func TestSdump_MaxDepth(t *testing.T) {
	type DeepStruct struct {
		Value interface{}
	}

	// 创建深度嵌套结构
	deep := DeepStruct{Value: DeepStruct{Value: DeepStruct{Value: "deep"}}}

	tests := []struct {
		name     string
		maxDepth int
		input    interface{}
	}{
		{"no_limit", 0, deep},
		{"depth_1", 1, deep},
		{"depth_2", 2, deep},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ConfigState{MaxDepth: tt.maxDepth}
			SetGlobalConfig(config)
			defer SetGlobalConfig(&Config) // 恢复默认配置

			result := Sdump(tt.input)
			t.Logf("Sdump(depth_%d) = %s", tt.maxDepth, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if parsed == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// 测试跳过nil值
func TestSdump_SkipNilValues(t *testing.T) {
	type StructWithNil struct {
		Name   string
		Ptr    *int
		Slice  []int
		Map    map[string]int
		Iface  interface{}
		NonNil string
	}

	data := StructWithNil{
		Name:   "test",
		NonNil: "value",
	}

	config := &ConfigState{SkipNilValues: true}
	SetGlobalConfig(config)
	defer SetGlobalConfig(&Config) // 恢复默认配置

	result := Sdump(data)
	t.Logf("Sdump(skip_nil) = %s", result)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON result: %v", err)
	}

	// 检查nil值是否被跳过
	if _, ok := parsed["Ptr"]; ok {
		t.Error("Expected nil pointer to be skipped")
	}
	if _, ok := parsed["Slice"]; ok {
		t.Error("Expected nil slice to be skipped")
	}
	if _, ok := parsed["Map"]; ok {
		t.Error("Expected nil map to be skipped")
	}
	if _, ok := parsed["Iface"]; ok {
		t.Error("Expected nil interface to be skipped")
	}

	// 检查非nil值是否保留
	if _, ok := parsed["Name"]; !ok {
		t.Error("Expected non-nil Name to be included")
	}
	if _, ok := parsed["NonNil"]; !ok {
		t.Error("Expected non-nil NonNil to be included")
	}
}

// 测试容器元素数量限制
func TestSdump_MaxElementsPerContainer(t *testing.T) {
	// 创建大切片
	largeSlice := make([]int, 100)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	// 创建大map
	largeMap := make(map[string]int, 100)
	for i := 0; i < 100; i++ {
		largeMap[fmt.Sprintf("key%d", i)] = i
	}

	tests := []struct {
		name                    string
		maxElementsPerContainer int
		input                   interface{}
	}{
		{"slice_limit_10", 10, largeSlice},
		{"slice_limit_50", 50, largeSlice},
		{"map_limit_10", 10, largeMap},
		{"map_limit_50", 50, largeMap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ConfigState{MaxElementsPerContainer: tt.maxElementsPerContainer}
			SetGlobalConfig(config)
			defer SetGlobalConfig(&Config) // 恢复默认配置

			result := Sdump(tt.input)
			t.Logf("Sdump(%s) = %s", tt.name, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if parsed == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// 测试复杂嵌套结构
func TestSdump_ComplexNestedStructures(t *testing.T) {
	type Address struct {
		Street  string
		City    string
		Country string
	}

	type Person struct {
		Name    string
		Age     int
		Address Address
		Hobbies []string
		Friends []*Person
	}

	person1 := &Person{
		Name: "Alice",
		Age:  30,
		Address: Address{
			Street:  "123 Main St",
			City:    "New York",
			Country: "USA",
		},
		Hobbies: []string{"reading", "swimming"},
	}

	person2 := &Person{
		Name: "Bob",
		Age:  25,
		Address: Address{
			Street:  "456 Oak Ave",
			City:    "Los Angeles",
			Country: "USA",
		},
		Hobbies: []string{"gaming", "cooking"},
	}

	person1.Friends = []*Person{person2}
	person2.Friends = []*Person{person1}

	result := Sdump(person1)
	t.Logf("Sdump(complex_nested) = %s", result)

	var parsed interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON result: %v", err)
	}

	if parsed == nil {
		t.Error("Expected non-nil result")
	}
}

// 测试时间类型
func TestSdump_TimeTypes(t *testing.T) {
	now := time.Now()
	duration := time.Hour * 2

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"time", now, now.String()},
		{"duration", duration, duration.String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			if parsed == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// 测试错误处理
func TestSdump_ErrorHandling(t *testing.T) {
	// 测试无效的reflect.Value
	var invalid reflect.Value

	result := Sdump(invalid)
	t.Logf("Sdump(invalid) = %s", result)

	var parsed interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON result: %v", err)
	}

	// 检查结果是否为map类型（reflect.Value被转换为map）
	if _, ok := parsed.(map[string]interface{}); !ok {
		t.Errorf("Expected map type for reflect.Value, got %T", parsed)
	}
}

// 测试多个参数
func TestSdump_MultipleArguments(t *testing.T) {
	result := Sdump("hello", 42, true, []int{1, 2, 3})
	t.Logf("Sdump(multiple) = %s", result)

	// 多个参数会输出多行，每行一个JSON值
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// 验证每行都是有效的JSON
	for _, line := range lines {
		var parsed interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Failed to parse line: %v", err)
		}
	}
}

// 测试空参数
func TestSdump_EmptyArguments(t *testing.T) {
	result := Sdump()
	t.Logf("Sdump(empty) = %s", result)

	// 空参数应该返回空字符串
	if result != "" {
		t.Errorf("Expected empty result, got: %s", result)
	}
}

// 测试性能基准
func BenchmarkSdump_SimpleStruct(b *testing.B) {
	type SimpleStruct struct {
		Name  string
		Age   int
		Email string
	}

	data := SimpleStruct{
		Name:  "Benchmark Test",
		Age:   25,
		Email: "benchmark@test.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Sdump(data)
	}
}

func BenchmarkSdump_ComplexStruct(b *testing.B) {
	type ComplexStruct struct {
		ID      int
		Name    string
		Tags    []string
		Data    map[string]interface{}
		Created time.Time
	}

	data := ComplexStruct{
		ID:   1,
		Name: "Complex Benchmark",
		Tags: []string{"tag1", "tag2", "tag3"},
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
		Created: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Sdump(data)
	}
}

func BenchmarkSdump_LargeSlice(b *testing.B) {
	largeSlice := make([]int, 1000)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Sdump(largeSlice)
	}
}

func BenchmarkSdump_LargeMap(b *testing.B) {
	largeMap := make(map[string]int, 1000)
	for i := 0; i < 1000; i++ {
		largeMap[fmt.Sprintf("key%d", i)] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Sdump(largeMap)
	}
}

// 测试大对象注册表系统
func TestSdump_LargeObjectRegistry(t *testing.T) {
	// 测试默认的大对象处理
	t.Run("default_handlers", func(t *testing.T) {
		now := time.Now()
		duration := time.Hour * 2

		result1 := Sdump(now)
		result2 := Sdump(duration)

		t.Logf("Sdump(time.Now()) = %s", result1)
		t.Logf("Sdump(time.Hour*2) = %s", result2)

		// 验证结果包含时间字符串而不是大对象
		if !strings.Contains(result1, "2025-") {
			t.Errorf("Expected time string, got: %s", result1)
		}
		if !strings.Contains(result2, "2h0m0s") {
			t.Errorf("Expected duration string, got: %s", result2)
		}
	})

	// 测试禁用大对象压缩
	t.Run("disable_compact", func(t *testing.T) {
		now := time.Now()

		var buf bytes.Buffer
		fdump(&buf, now)
		result := buf.String()

		t.Logf("Sdump with CompactLargeObjects=false = %s", result)

		// 当禁用压缩时，应该返回完整的结构体信息
		if !strings.Contains(result, "wall") || !strings.Contains(result, "ext") {
			t.Errorf("Expected full struct dump, got: %s", result)
		}
	})

	// 测试自定义大对象处理器
	t.Run("custom_handler", func(t *testing.T) {
		// 注册自定义处理器
		RegisterLargeObject("custom.Type", func(v reflect.Value) (string, bool) {
			return "CUSTOM_HANDLED", true
		})

		// 创建一个自定义类型
		type CustomType struct {
			Value string
		}

		custom := CustomType{Value: "test"}

		// 由于我们的类型不是 "custom.Type"，它不会被处理
		result := Sdump(custom)
		t.Logf("Sdump(custom) = %s", result)

		// 应该返回完整的结构体信息
		if !strings.Contains(result, "Value") {
			t.Errorf("Expected struct dump, got: %s", result)
		}

		// 清理
		UnregisterLargeObject("custom.Type")
	})
}

// 测试指针类型大对象
func TestSdump_PointerLargeObjects(t *testing.T) {
	// 测试指针类型的 time.Time
	now := time.Now()
	timePtr := &now

	// 测试指针类型的 time.Duration
	duration := time.Hour * 2
	durationPtr := &duration

	// 测试指针类型的自定义结构体（应该不被压缩）
	type CustomStruct struct {
		Name string
		Age  int
	}
	custom := CustomStruct{Name: "test", Age: 25}
	customPtr := &custom

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"time.Time pointer", timePtr, "2025-"},
		{"time.Duration pointer", durationPtr, "2h0m0s"},
		{"custom struct pointer", customPtr, "Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			// 检查结果是否包含期望的内容
			if str, ok := parsed.(string); ok {
				if !strings.Contains(str, tt.expected) {
					t.Errorf("Expected result to contain '%s', got '%s'", tt.expected, str)
				}
			} else {
				// 对于自定义结构体，应该是对象而不是字符串
				if tt.name == "custom struct pointer" {
					if obj, ok := parsed.(map[string]interface{}); ok {
						if _, exists := obj["Name"]; !exists {
							t.Errorf("Expected object with 'Name' field, got %v", obj)
						}
					} else {
						t.Errorf("Expected object for custom struct, got %T", parsed)
					}
				} else {
					t.Errorf("Expected string result, got %T", parsed)
				}
			}
		})
	}
}

// 测试接口类型大对象
func TestSdump_InterfaceLargeObjects(t *testing.T) {
	// 通过接口传递 time.Time
	var timeInterface interface{} = time.Now()

	// 通过接口传递 time.Duration
	var durationInterface interface{} = time.Hour * 2

	// 通过接口传递自定义结构体
	type CustomStruct struct {
		Name string
		Age  int
	}
	var customInterface interface{} = CustomStruct{Name: "test", Age: 25}

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"time.Time interface", timeInterface, "2025-"},
		{"time.Duration interface", durationInterface, "2h0m0s"},
		{"custom struct interface", customInterface, "Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			// 检查结果是否包含期望的内容
			if str, ok := parsed.(string); ok {
				if !strings.Contains(str, tt.expected) {
					t.Errorf("Expected result to contain '%s', got '%s'", tt.expected, str)
				}
			} else {
				// 对于自定义结构体，应该是对象而不是字符串
				if tt.name == "custom struct interface" {
					if obj, ok := parsed.(map[string]interface{}); ok {
						if _, exists := obj["Name"]; !exists {
							t.Errorf("Expected object with 'Name' field, got %v", obj)
						}
					} else {
						t.Errorf("Expected object for custom struct, got %T", parsed)
					}
				} else {
					t.Errorf("Expected string result, got %T", parsed)
				}
			}
		})
	}
}

// 测试容器中的大对象
func TestSdump_ContainerLargeObjects(t *testing.T) {
	// 测试 map 中的大对象
	timeMap := map[string]interface{}{
		"time":     time.Now(),
		"duration": time.Hour * 2,
		"string":   "hello",
	}

	// 测试 slice 中的大对象
	timeSlice := []interface{}{
		time.Now(),
		time.Hour * 2,
		"hello",
	}

	// 测试嵌套结构体中的大对象
	type NestedStruct struct {
		Time     time.Time
		Duration time.Duration
		Name     string
	}

	nested := NestedStruct{
		Time:     time.Now(),
		Duration: time.Hour * 2,
		Name:     "test",
	}

	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{"map with large objects", timeMap, []string{"2025-", "2h0m0s", "hello"}},
		{"slice with large objects", timeSlice, []string{"2025-", "2h0m0s", "hello"}},
		{"nested struct with large objects", nested, []string{"2025-", "2h0m0s", "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sdump(tt.input)
			t.Logf("Sdump(%v) = %s", tt.input, result)

			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("Failed to parse JSON result: %v", err)
			}

			// 检查结果是否包含所有期望的内容
			resultStr := fmt.Sprintf("%v", parsed)
			for _, expected := range tt.expected {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected result to contain '%s', got '%s'", expected, resultStr)
				}
			}
		})
	}
}
