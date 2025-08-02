package objectdump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

// 测试用的结构体
type TestStruct struct {
	Name       string
	Age        int
	Active     bool
	Ptr        *int
	Slice      []string
	Map        map[string]interface{}
	unexported string // 未导出字段
}

// 循环引用结构体
type CircularStruct struct {
	Name string
	Next *CircularStruct
}

// 实现Stringer接口的结构体
type StringerStruct struct {
	Value string
}

func (s StringerStruct) String() string {
	return "Stringer:" + s.Value
}

// 测试基本类型
func TestBasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"int", 42, int64(42)},
		{"int8", int8(42), int64(42)},
		{"int16", int16(42), int64(42)},
		{"int32", int32(42), int64(42)},
		{"int64", int64(42), int64(42)},
		{"uint", uint(42), uint64(42)},
		{"uint8", uint8(42), uint64(42)},
		{"uint16", uint16(42), uint64(42)},
		{"uint32", uint32(42), uint64(42)},
		{"uint64", uint64(42), uint64(42)},
		{"float32", float32(3.14), float64(3.14)},
		{"float64", 3.14, 3.14},
		{"string", "hello", "hello"},
		{"complex64", complex64(1 + 2i), "1+2i"},
		{"complex128", complex128(1 + 2i), "1+2i"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dumpState{cs: &Config}
			result := d.dump(reflect.ValueOf(tt.input))
			if result != tt.expected {
				t.Errorf("dump(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试特殊浮点值
func TestSpecialFloatValues(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected interface{}
	}{
		{"NaN", math.NaN(), "NaN"},
		{"Inf", math.Inf(1), "Inf"},
		{"-Inf", math.Inf(-1), "-Inf"},
		{"normal", 3.14, 3.14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dumpState{cs: &Config}
			result := d.dump(reflect.ValueOf(tt.input))
			if result != tt.expected {
				t.Errorf("dump(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试指针类型
func TestPointerTypes(t *testing.T) {
	value := 42
	ptr := &value
	var nilPtr *int

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"nil pointer", nilPtr, nil},
		{"valid pointer", ptr, int64(42)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dumpState{cs: &Config}
			result := d.dump(reflect.ValueOf(tt.input))
			if result != tt.expected {
				t.Errorf("dump(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试循环引用
func TestCircularReference(t *testing.T) {
	circular := &CircularStruct{Name: "root"}
	circular.Next = circular

	d := &dumpState{cs: &Config}
	result := d.dump(reflect.ValueOf(circular))

	// 检查结果是否包含循环引用信息
	resultStr := fmt.Sprintf("%v \n", result)
	t.Logf("resultStr: %s", resultStr)
	if !strings.Contains(resultStr, "circular") {
		t.Errorf("Expected circular reference detection, got: %v", result)
	}
}

// 测试结构体
func TestStructType(t *testing.T) {
	value := 42
	testStruct := TestStruct{
		Name:       "test",
		Age:        25,
		Active:     true,
		Ptr:        &value,
		Slice:      []string{"a", "b", "c"},
		Map:        map[string]interface{}{"key": "value"},
		unexported: "private",
	}

	d := &dumpState{cs: &Config}
	result := d.dump(reflect.ValueOf(testStruct))

	// 验证结果类型
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}
	t.Logf("resultMap: %s", result)
	// 验证字段
	if resultMap["Name"] != "test" {
		t.Errorf("Expected Name=test, got %v", resultMap["Name"])
	}
	if resultMap["Age"] != int64(25) {
		t.Errorf("Expected Age=25, got %v", resultMap["Age"])
	}
	if resultMap["Active"] != true {
		t.Errorf("Expected Active=true, got %v", resultMap["Active"])
	}
	if resultMap["unexported"] != "<unexported>" {
		t.Errorf("Expected unexported=<unexported>, got %v", resultMap["unexported"])
	}
}

// 测试未导出字段访问
func TestUnexportedFields(t *testing.T) {
	testStruct := TestStruct{
		Name:       "test",
		unexported: "private",
	}

	config := &ConfigState{AllowUnexported: true}
	d := &dumpState{cs: config}
	result := d.dump(reflect.ValueOf(testStruct))

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	// 应该能够访问未导出字段
	if resultMap["unexported"] != "private" {
		t.Errorf("Expected unexported=private, got %v", resultMap["unexported"])
	}
}

// 测试切片和数组
func TestSliceAndArray(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	array := [3]string{"a", "b", "c"}

	tests := []struct {
		name        string
		input       interface{}
		expectedLen int
	}{
		{"slice", slice, 5},
		{"array", array, 3},
		{"nil slice", []int(nil), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dumpState{cs: &Config}
			result := d.dump(reflect.ValueOf(tt.input))

			if tt.name == "nil slice" {
				if result != nil {
					t.Errorf("Expected nil for nil slice, got %v", result)
				}
				return
			}

			resultSlice, ok := result.([]interface{})
			if !ok {
				t.Fatalf("Expected slice result, got %T", result)
			}

			if len(resultSlice) != tt.expectedLen {
				t.Errorf("Expected length %d, got %d", tt.expectedLen, len(resultSlice))
			}
		})
	}
}

// 测试字节切片
func TestByteSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"printable ascii", []byte("hello"), "hello"},
		{"hex bytes", []byte{0x00, 0xFF, 0x1A}, "00ff1a"},
		{"json", []byte(`{"key":"value"}`), `{"key":"value"}`},
		{"empty", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dumpState{cs: &Config}
			result := d.dump(reflect.ValueOf(tt.input))
			if result != tt.expected {
				t.Errorf("dump(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试Map
func TestMap(t *testing.T) {
	testMap := map[string]interface{}{
		"string": "value",
		"int":    42,
		"bool":   true,
		"slice":  []int{1, 2, 3},
	}

	d := &dumpState{cs: &Config}
	result := d.dump(reflect.ValueOf(testMap))

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	// 验证键值对
	if resultMap["string"] != "value" {
		t.Errorf("Expected string=value, got %v", resultMap["string"])
	}
	if resultMap["int"] != int64(42) {
		t.Errorf("Expected int=42, got %v", resultMap["int"])
	}
}

// 测试nil值
func TestNilValues(t *testing.T) {
	var nilSlice []int
	var nilMap map[string]int
	var nilPtr *int
	var nilInterface interface{}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"nil slice", nilSlice, nil},
		{"nil map", nilMap, nil},
		{"nil pointer", nilPtr, nil},
		{"nil interface", nilInterface, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dumpState{cs: &Config}
			result := d.dump(reflect.ValueOf(tt.input))
			if result != tt.expected {
				t.Errorf("dump(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试SkipNilValues配置
func TestSkipNilValues(t *testing.T) {
	testStruct := TestStruct{
		Name: "test",
		// 其他字段保持零值（nil）
	}

	config := &ConfigState{SkipNilValues: true}
	d := &dumpState{cs: config}
	result := d.dump(reflect.ValueOf(testStruct))

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	// 应该跳过nil值
	if _, exists := resultMap["Ptr"]; exists {
		t.Error("Expected Ptr to be skipped, but it exists")
	}
	if _, exists := resultMap["Slice"]; exists {
		t.Error("Expected Slice to be skipped, but it exists")
	}
	if _, exists := resultMap["Map"]; exists {
		t.Error("Expected Map to be skipped, but it exists")
	}

	// 非nil值应该存在
	if resultMap["Name"] != "test" {
		t.Errorf("Expected Name=test, got %v", resultMap["Name"])
	}
}

// 测试最大深度限制
func TestMaxDepth(t *testing.T) {
	nested := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": "deep",
			},
		},
	}

	config := &ConfigState{MaxDepth: 1}
	d := &dumpState{cs: config}
	result := d.dump(reflect.ValueOf(nested))

	resultStr := fmt.Sprintf("%v", result)
	if !strings.Contains(resultStr, "truncated") {
		t.Errorf("Expected truncated output, got: %v", result)
	}
}

// 测试容器元素数量限制
func TestMaxElementsPerContainer(t *testing.T) {
	largeSlice := make([]int, 1500)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	config := &ConfigState{MaxElementsPerContainer: 100}
	d := &dumpState{cs: config}
	result := d.dump(reflect.ValueOf(largeSlice))

	resultSlice, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected slice result, got %T", result)
	}

	// 应该被截断到100个元素加上截断信息
	if len(resultSlice) != 101 {
		t.Errorf("Expected 101 elements (100 + truncation info), got %d", len(resultSlice))
	}

	// 最后一个元素应该是截断信息
	lastElem, ok := resultSlice[100].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected truncation info as map, got %T", resultSlice[100])
	}

	if lastElem["__truncated__"] != true {
		t.Error("Expected __truncated__ flag to be true")
	}
}

// 测试公共API函数
func TestPublicAPIs(t *testing.T) {
	testValue := "hello"

	// 测试Sdump
	result := Sdump(testValue)
	if !strings.Contains(result, "hello") {
		t.Errorf("Sdump failed, got: %s", result)
	}

	// 测试ToJSON
	jsonResult := ToJSON(testValue)
	if !strings.Contains(jsonResult, "hello") {
		t.Errorf("ToJSON failed, got: %s", jsonResult)
	}

	// 测试SdumpJSON
	jsonResult2 := SdumpJSON(testValue)
	if jsonResult2 != jsonResult {
		t.Errorf("SdumpJSON should equal ToJSON, got: %s vs %s", jsonResult2, jsonResult)
	}

	// 测试ConfigState.ToJSON
	config := &ConfigState{Indent: "  "}
	jsonResult3 := config.ToJSON(testValue)
	if !strings.Contains(jsonResult3, "hello") {
		t.Errorf("ConfigState.ToJSON failed, got: %s", jsonResult3)
	}
}

// 测试Fdump
func TestFdump(t *testing.T) {
	var buf bytes.Buffer
	testValue := "test"

	Fdump(&buf, testValue)
	result := buf.String()

	if !strings.Contains(result, "test") {
		t.Errorf("Fdump failed, got: %s", result)
	}
}

// 测试内存池
func TestMemoryPool(t *testing.T) {
	// 测试GetPoolStats
	inUse, available := GetPoolStats()
	if inUse < 0 || available < 0 {
		t.Errorf("Invalid pool stats: inUse=%d, available=%d", inUse, available)
	}

	// 测试内存池的获取和归还
	config := &ConfigState{}
	d1 := getDumpState(config)
	if d1 == nil {
		t.Fatal("getDumpState returned nil")
	}

	putDumpState(d1)

	d2 := getDumpState(config)
	if d2 == nil {
		t.Fatal("getDumpState returned nil after putDumpState")
	}

	putDumpState(d2)
}

// 测试DumpToJSON
func TestDumpToJSON(t *testing.T) {
	testValue := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	config := &ConfigState{}
	result, err := DumpToJSON(testValue, config)
	if err != nil {
		t.Fatalf("DumpToJSON failed: %v", err)
	}

	// 验证JSON格式
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if parsed["key"] != "value" {
		t.Errorf("Expected key=value, got %v", parsed["key"])
	}
}

// 测试工具函数
func TestUtilityFunctions(t *testing.T) {
	// 测试isDeepType
	if !isDeepType(reflect.Ptr) {
		t.Error("Expected Ptr to be deep type")
	}
	if !isDeepType(reflect.Struct) {
		t.Error("Expected Struct to be deep type")
	}
	if !isDeepType(reflect.Map) {
		t.Error("Expected Map to be deep type")
	}
	if !isDeepType(reflect.Slice) {
		t.Error("Expected Slice to be deep type")
	}
	if !isDeepType(reflect.Array) {
		t.Error("Expected Array to be deep type")
	}
	if isDeepType(reflect.String) {
		t.Error("Expected String to not be deep type")
	}

	// 测试isPrintableOrControlASCII
	if !isPrintableOrControlASCII([]byte("hello")) {
		t.Error("Expected printable ASCII to be true")
	}
	if !isPrintableOrControlASCII([]byte("hello\n\t\r")) {
		t.Error("Expected printable ASCII with control chars to be true")
	}
	if isPrintableOrControlASCII([]byte{0x00, 0xFF}) {
		t.Error("Expected non-printable bytes to be false")
	}

	// 测试isJSON
	if !isJSON([]byte(`{"key":"value"}`)) {
		t.Error("Expected JSON object to be true")
	}
	if !isJSON([]byte(`["item1","item2"]`)) {
		t.Error("Expected JSON array to be true")
	}
	if isJSON([]byte("not json")) {
		t.Error("Expected non-JSON to be false")
	}

	// 测试isUTF8
	if !isUTF8([]byte("hello")) {
		t.Error("Expected valid UTF-8 to be true")
	}
	if isUTF8([]byte{0xFF, 0xFE}) {
		t.Error("Expected invalid UTF-8 to be false")
	}
}

// 测试getUint8String
func TestGetUint8String(t *testing.T) {
	tests := []struct {
		name     string
		input    []uint8
		expected string
	}{
		{"empty", []uint8{}, ""},
		{"printable ascii", []uint8("hello"), "hello"},
		{"hex bytes", []uint8{0x00, 0xFF, 0x1A}, `["00 ff", "1a"]`},
		{"large buffer", make([]uint8, 3000), string(make([]uint8, 2048))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getUint8String(tt.input)
			if result != tt.expected {
				t.Errorf("getUint8String(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试truncatedContainerInfo
func TestTruncatedContainerInfo(t *testing.T) {
	result1 := truncatedContainerInfo("[]int", 5, 3, 10, 20)
	if !strings.Contains(result1, "truncated") {
		t.Errorf("Expected truncated info, got: %s", result1)
	}

	result2 := truncatedContainerInfo("[]int", 5, 3, 0, 0)
	if !strings.Contains(result2, "truncated") {
		t.Errorf("Expected truncated info, got: %s", result2)
	}
}

// 测试无效值
func TestInvalidValue(t *testing.T) {
	var invalid reflect.Value
	d := &dumpState{cs: &Config}
	result := d.dump(invalid)
	if result != "invalid" {
		t.Errorf("Expected 'invalid', got %v", result)
	}
}

// 测试Channel和Function类型
func TestChannelAndFunction(t *testing.T) {
	ch := make(chan int)
	fn := func() {}

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"channel", ch, "<chan chan int"},
		{"function", fn, "<func func()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dumpState{cs: &Config}
			result := d.dump(reflect.ValueOf(tt.input))
			resultStr := fmt.Sprintf("%v", result)
			if !strings.Contains(resultStr, tt.expected[:len(tt.expected)-1]) {
				t.Errorf("dump(%v) = %v, want to contain %v", tt.input, resultStr, tt.expected)
			}
		})
	}
}

// 测试UnsafePointer
func TestUnsafePointer(t *testing.T) {
	ptr := unsafe.Pointer(uintptr(0x12345678))
	d := &dumpState{cs: &Config}
	result := d.dump(reflect.ValueOf(ptr))

	resultStr := fmt.Sprintf("%v", result)
	if !strings.Contains(resultStr, "unsafe.Pointer") {
		t.Errorf("Expected unsafe.Pointer info, got: %v", result)
	}
}

// 测试bypassUnsafeReflectValue
func TestBypassUnsafeReflectValue(t *testing.T) {
	testStruct := TestStruct{
		Name:       "test",
		unexported: "private",
	}

	field, _ := reflect.TypeOf(testStruct).FieldByName("unexported")
	result := bypassUnsafeReflectValue(field, reflect.ValueOf(testStruct))

	if result.String() != "private" {
		t.Errorf("Expected 'private', got %v", result.String())
	}
}

// 测试不可寻址的值
func TestNonAddressableValue(t *testing.T) {
	testStruct := TestStruct{Name: "test"}
	field, _ := reflect.TypeOf(testStruct).FieldByName("unexported")

	// 通过接口传递，使其不可寻址
	var i interface{} = testStruct
	result := bypassUnsafeReflectValue(field, reflect.ValueOf(i))

	if result.String() != "<unexported, not addressable>" {
		t.Errorf("Expected '<unexported, not addressable>', got %v", result.String())
	}
}

// 测试Dump函数（输出到stdout）
func TestDump(t *testing.T) {
	// 这个测试主要是确保函数不会panic
	// 由于它输出到stdout，我们无法轻易捕获输出
	testValue := "test"

	// 应该不会panic
	Dump(testValue)
}

// 测试边界情况
func TestEdgeCases(t *testing.T) {
	// 测试空结构体
	emptyStruct := struct{}{}
	d := &dumpState{cs: &Config}
	result := d.dump(reflect.ValueOf(emptyStruct))

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	// 应该只包含type字段
	if len(resultMap) != 1 || resultMap["type"] == nil {
		t.Errorf("Expected only type field, got %v", resultMap)
	}

	// 测试空map
	emptyMap := map[string]int{}
	result = d.dump(reflect.ValueOf(emptyMap))

	resultMap2, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	if len(resultMap2) != 0 {
		t.Errorf("Expected empty map, got %v", resultMap2)
	}
}

// 测试JSON编码器配置
func TestJSONEncoderConfig(t *testing.T) {
	testValue := map[string]string{"key": "value&<test>"}

	var buf bytes.Buffer
	fdump(&buf, testValue)

	result := buf.String()
	// 检查HTML字符是否被转义
	if strings.Contains(result, "&amp;") {
		t.Error("Expected HTML characters to not be escaped")
	}
}

// 测试并发安全性
func TestConcurrency(t *testing.T) {
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			testValue := map[string]int{
				"goroutine": id,
				"data":      id * 2,
			}

			result := ToJSON(testValue)
			if !strings.Contains(result, fmt.Sprintf("%d", id)) {
				t.Errorf("Goroutine %d: Expected result to contain %d", id, id)
			}
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// 基准测试
func BenchmarkDump(b *testing.B) {
	testValue := map[string]interface{}{
		"string": "value",
		"int":    42,
		"bool":   true,
		"slice":  []int{1, 2, 3, 4, 5},
		"map": map[string]interface{}{
			"nested": "value",
			"array":  []string{"a", "b", "c"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := &dumpState{cs: &Config}
		d.dump(reflect.ValueOf(testValue))
	}
}

func BenchmarkToJSON(b *testing.B) {
	testValue := map[string]interface{}{
		"string": "value",
		"int":    42,
		"bool":   true,
		"slice":  []int{1, 2, 3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToJSON(testValue)
	}
}

func BenchmarkMemoryPool(b *testing.B) {
	config := &ConfigState{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := getDumpState(config)
		putDumpState(d)
	}
}
