/*
 * Copyright (c) 2013-2016 Dave Collins <dave@davec.name>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

/*
JSON Output Test Summary:
This test suite validates the JSON output functionality for all Go data types:

- Basic types: int, uint, bool, float, complex, string
- Composite types: array, slice, map, struct
- Special types: pointer, interface, channel, function, uintptr
- Edge cases: nil values, circular references, invalid types
- Configuration options: SkipNilValues, MaxDepth
*/

package spew

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	gspew "github.com/davecgh/go-spew/spew"
)

// TestBasicTypesJSON tests JSON output for basic Go types
func TestBasicTypesJSON(t *testing.T) {
	// 使用变量而不是常量
	var int8Var = int8(127)
	var int16Var = int16(32767)
	var int32Var = int32(2147483647)
	var int64Var = int64(9223372036854775807)
	var intVar = int(123)

	var uint8Var = uint8(255)
	var uint16Var = uint16(65535)
	var uint32Var = uint32(4294967295)
	var uint64Var = uint64(18446744073709551615)
	var uintVar = uint(456)

	var float32Var = float32(3.14)
	var float64Var = float64(2.718281828)

	var boolTrueVar = true
	var boolFalseVar = false

	var stringVar = "Hello, World!"
	var emptyStringVar = ""

	var complex64Var = complex64(1 + 2i)
	var complex128Var = complex128(3 + 4i)

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		// Integer types (JSON parses all numbers as float64)
		{"int8", int8Var, float64(127)},
		{"int16", int16Var, float64(32767)},
		{"int32", int32Var, float64(2147483647)},
		{"int64", int64Var, float64(9223372036854775807)},
		{"int", intVar, float64(123)},

		// Unsigned integer types (JSON parses all numbers as float64)
		{"uint8", uint8Var, float64(255)},
		{"uint16", uint16Var, float64(65535)},
		{"uint32", uint32Var, float64(4294967295)},
		{"uint64", uint64Var, float64(18446744073709551615)},
		{"uint", uintVar, float64(456)},

		// Float types
		{"float32", float32Var, nil}, // Special case - will check separately due to precision
		{"float64", float64Var, float64(2.718281828)},

		// Boolean types
		{"bool_true", boolTrueVar, true},
		{"bool_false", boolFalseVar, false},

		// String type
		{"string", stringVar, "Hello, World!"},
		{"empty_string", emptyStringVar, ""},

		// Complex types (converted to string in JSON)
		{"complex64", complex64Var, "(1+2i)"},
		{"complex128", complex128Var, "(3+4i)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestBasicTypesJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// Check that it contains a value field
			value, exists := parsed["value"]
			if !exists {
				t.Errorf("TestBasicTypesJSON[%s] missing 'value' field", tt.name)
				return
			}

			// Special handling for float32 due to precision issues
			if tt.name == "float32" {
				if floatVal, ok := value.(float64); ok {
					expectedFloat32 := float64(float32(3.14)) // Convert through float32 to get the same precision
					if abs(floatVal-expectedFloat32) > 1e-6 {
						t.Errorf("TestBasicTypesJSON[%s] float32 precision issue\nexpected: %v\nactual: %v",
							tt.name, expectedFloat32, floatVal)
					}
				} else {
					t.Errorf("TestBasicTypesJSON[%s] expected float64, got %T", tt.name, value)
				}
				return
			}

			// Verify the value matches expected for other types
			if value != tt.expected {
				t.Errorf("TestBasicTypesJSON[%s]\nexpected value: %v (%T)\nactual value:   %v (%T)",
					tt.name, tt.expected, tt.expected, value, value)
			}
		})
	}
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestArraySliceJSON tests JSON output for arrays and slices
func TestArraySliceJSON(t *testing.T) {
	// 使用变量而不是常量
	var arrayInt = [3]int{1, 2, 3}
	var arrayString = [2]string{"hello", "world"}
	var emptyArray = [0]int{}

	var sliceInt = []int{10, 20, 30}
	var sliceString = []string{"foo", "bar"}
	var emptySlice = []int{}
	var nilSlice []int = nil

	tests := []struct {
		name  string
		input interface{}
	}{
		// Arrays
		{"array_int", arrayInt},
		{"array_string", arrayString},
		{"empty_array", emptyArray},

		// Slices
		{"slice_int", sliceInt},
		{"slice_string", sliceString},
		{"empty_slice", emptySlice},
		{"nil_slice", nilSlice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestArraySliceJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// Check if it contains the expected value structure
			if tt.input == nil {
				if parsed["value"] != nil {
					t.Errorf("TestArraySliceJSON[%s] expected null value, got: %v", tt.name, parsed["value"])
				}
			} else {
				// For non-nil values, should have some representation
				if _, exists := parsed["value"]; !exists {
					t.Errorf("TestArraySliceJSON[%s] missing 'value' field", tt.name)
				}
			}
		})
	}
}

// TestMapJSON tests JSON output for maps
func TestMapJSON(t *testing.T) {
	// 使用变量而不是常量
	var mapStringInt = map[string]int{"one": 1, "two": 2}
	var mapIntString = map[int]string{1: "one", 2: "two"}
	var emptyMap = map[string]int{}
	var nilMap map[string]int = nil

	tests := []struct {
		name  string
		input interface{}
	}{
		{"map_string_int", mapStringInt},
		{"map_int_string", mapIntString},
		{"empty_map", emptyMap},
		{"nil_map", nilMap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestMapJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// Check basic structure
			if tt.input == nil {
				if parsed["value"] != nil {
					t.Errorf("TestMapJSON[%s] expected null value for nil map", tt.name)
				}
			} else {
				if _, exists := parsed["value"]; !exists {
					t.Errorf("TestMapJSON[%s] missing 'value' field", tt.name)
				}
			}
		})
	}
}

// TestStructJSON tests JSON output for structs
func TestStructJSON(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	type Company struct {
		Name      string
		Employees []Person
	}

	// 使用变量而不是常量
	var simplePerson = Person{Name: "John", Age: 30}
	var nestedCompany = Company{
		Name: "Tech Corp",
		Employees: []Person{
			{Name: "Alice", Age: 25},
			{Name: "Bob", Age: 35},
		},
	}
	var emptyStruct = struct{}{}

	tests := []struct {
		name  string
		input interface{}
	}{
		{"simple_struct", simplePerson},
		{"nested_struct", nestedCompany},
		{"empty_struct", emptyStruct},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestStructJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// Check that struct has type information
			if parsed["value"] == nil {
				t.Errorf("TestStructJSON[%s] expected non-null value", tt.name)
			}
		})
	}
}

// TestPointerJSON tests JSON output for pointers
func TestPointerJSON(t *testing.T) {
	// 使用变量而不是常量
	var value = 42
	var nilPtr *int = nil
	var ptrToPtr = &nilPtr

	tests := []struct {
		name  string
		input interface{}
	}{
		{"pointer_to_int", &value},
		{"nil_pointer", nilPtr},
		{"pointer_to_pointer", ptrToPtr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestPointerJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestInterfaceJSON tests JSON output for interfaces
func TestInterfaceJSON(t *testing.T) {
	// 使用变量而不是常量
	var nilInterface interface{} = nil
	var stringInterface interface{} = "hello"
	var intInterface interface{} = 123

	tests := []struct {
		name  string
		input interface{}
	}{
		{"nil_interface", nilInterface},
		{"string_interface", stringInterface},
		{"int_interface", intInterface},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestInterfaceJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestSpecialTypesJSON tests JSON output for special types
func TestSpecialTypesJSON(t *testing.T) {
	// 使用变量而不是常量
	var ch = make(chan int, 1)
	var fn = func() int { return 42 }
	var ptr = uintptr(unsafe.Pointer(&ch))
	var unsafePtr = unsafe.Pointer(&ch)

	tests := []struct {
		name  string
		input interface{}
	}{
		{"channel", ch},
		{"function", fn},
		{"uintptr", ptr},
		{"unsafe_pointer", unsafePtr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestSpecialTypesJSON[%s] produced invalid JSON: %v", tt.name, err)
			}

			// Special types should have value field
			if parsed["value"] == nil {
				t.Errorf("TestSpecialTypesJSON[%s] expected non-null value", tt.name)
			}
		})
	}
}

// TestByteSliceJSON tests JSON output for byte slices
func TestByteSliceJSON(t *testing.T) {
	// 使用变量而不是常量
	var byteSliceAscii = []byte("Hello")
	var byteSliceBinary = []byte{0x01, 0x02, 0x03, 0x04}
	var emptyByteSlice = []byte{}
	var nilByteSlice []byte = nil

	tests := []struct {
		name  string
		input interface{}
	}{
		{"byte_slice_ascii", byteSliceAscii},
		{"byte_slice_binary", byteSliceBinary},
		{"empty_byte_slice", emptyByteSlice},
		{"nil_byte_slice", nilByteSlice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestByteSliceJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestByteSliceSmartJSON tests smart byte slice handling for JSON output
func TestByteSliceSmartJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectASCII bool // 是否期望ASCII输出
		description string
	}{
		{
			name:        "pure_ascii_text",
			input:       []byte("Hello, World!"),
			expectASCII: true,
			description: "纯ASCII文本应该直接显示",
		},
		{
			name:        "text_with_newline",
			input:       []byte("Line1\nLine2\tTabbed"),
			expectASCII: true,
			description: "包含换行符和制表符的文本应该直接显示",
		},
		{
			name:        "binary_data",
			input:       []byte{0x00, 0x01, 0xFF, 0xFE},
			expectASCII: false,
			description: "纯二进制数据应该显示为十六进制",
		},
		{
			name:        "mixed_data",
			input:       []byte{'H', 'e', 'l', 'l', 'o', 0x00, 0xFF},
			expectASCII: false,
			description: "混合数据（ASCII+二进制）应该显示为十六进制",
		},
		{
			name:        "json_text",
			input:       []byte(`{"key": "value"}`),
			expectASCII: true,
			description: "JSON文本应该直接显示",
		},
		{
			name:        "utf8_text",
			input:       []byte("Hello 世界"),
			expectASCII: false,
			description: "UTF-8文本（包含非ASCII字符）应该显示为十六进制",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Printf("%s: %s\n", tt.description, result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestByteSliceSmartJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// 检查value字段
			value, exists := parsed["value"]
			if !exists {
				t.Errorf("TestByteSliceSmartJSON[%s] missing 'value' field", tt.name)
				return
			}

			if tt.expectASCII {
				// 期望ASCII输出：应该是字符串类型
				if strValue, ok := value.(string); ok {
					expectedStr := string(tt.input)
					if strValue != expectedStr {
						t.Errorf("TestByteSliceSmartJSON[%s] ASCII output mismatch\nexpected: %q\nactual: %q",
							tt.name, expectedStr, strValue)
					}
				} else {
					t.Errorf("TestByteSliceSmartJSON[%s] expected string output for ASCII data, got %T",
						tt.name, value)
				}
			} else {
				// 期望十六进制输出：应该是字符串且内容为十六进制
				if strValue, ok := value.(string); ok {
					// 检查是否为合法十六进制字符串
					if _, err := hex.DecodeString(strValue); err != nil {
						t.Errorf("TestByteSliceSmartJSON[%s] expected hex format output, got: %s",
							tt.name, strValue)
					}
				} else {
					t.Errorf("TestByteSliceSmartJSON[%s] expected string output for hex data, got %T",
						tt.name, value)
				}
			}
		})
	}
}

// TestConfigStateJSON tests JSON output with different configurations
func TestConfigStateJSON(t *testing.T) {
	type TestStruct struct {
		Name     string
		Value    *int
		NilField *string
	}

	value := 42
	data := TestStruct{
		Name:     "test",
		Value:    &value,
		NilField: nil,
	}

	t.Run("default_config", func(t *testing.T) {
		result := ToJSON(data)
		result = strings.TrimSpace(result)

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("TestConfigStateJSON[default_config] produced invalid JSON: %v", err)
		}
	})

	t.Run("skip_nil_values", func(t *testing.T) {
		cs := ConfigState{
			SkipNilValues: true,
		}
		result := cs.ToJSON(data)
		result = strings.TrimSpace(result)

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("TestConfigStateJSON[skip_nil_values] produced invalid JSON: %v", err)
		}
	})

	t.Run("max_depth", func(t *testing.T) {
		cs := ConfigState{
			MaxDepth: 2,
		}
		result := cs.ToJSON(data)
		result = strings.TrimSpace(result)

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("TestConfigStateJSON[max_depth] produced invalid JSON: %v", err)
		}
	})
}

// TestCircularReferenceJSON tests JSON output for circular references
func TestCircularReferenceJSON(t *testing.T) {
	type Node struct {
		Name string
		Next *Node
	}

	node1 := &Node{Name: "Node1"}
	node2 := &Node{Name: "Node2"}
	node1.Next = node2
	node2.Next = node1 // Create circular reference

	t.Run("circular_reference", func(t *testing.T) {
		result := ToJSON(node1)
		result = strings.TrimSpace(result)

		// Should not panic and should produce valid JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("TestCircularReferenceJSON produced invalid JSON: %v", err)
		}

		// Should contain some result
		if result == "" {
			t.Errorf("TestCircularReferenceJSON produced empty result")
		}
	})
}

// TestErrorCasesJSON tests JSON output for error cases
func TestErrorCasesJSON(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		result := ToJSON(nil)
		result = strings.TrimSpace(result)

		// Should produce valid JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("TestErrorCasesJSON[nil_input] produced invalid JSON: %v", err)
		}
	})

	t.Run("multiple_values", func(t *testing.T) {
		result := ToJSON(1, 2, 3)
		lines := strings.Split(strings.TrimSpace(result), "\n")

		if len(lines) < 3 {
			t.Errorf("TestErrorCasesJSON[multiple_values] expected at least 3 lines, got %d", len(lines))
		}

		// Each line should be valid JSON
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(line), &parsed); err != nil {
				t.Errorf("TestErrorCasesJSON[multiple_values] line %d produced invalid JSON: %v", i, err)
			}
		}
	})
}

// TestJSONValidation ensures all outputs are valid JSON
func TestJSONValidation(t *testing.T) {
	testInputs := []interface{}{
		// Basic types
		42, "hello", true, 3.14, complex(1, 2),

		// Collections
		[]int{1, 2, 3}, map[string]int{"a": 1}, [2]string{"x", "y"},

		// Pointers and nil
		func() *int { i := 42; return &i }(), (*int)(nil),

		// Special types
		make(chan int), func() {}, uintptr(0x1234),

		// Byte slices
		[]byte("test"), []byte{1, 2, 3, 4},
	}

	for i, input := range testInputs {
		t.Run(fmt.Sprintf("validation_%d", i), func(t *testing.T) {
			result := ToJSON(input)
			result = strings.TrimSpace(result)
			fmt.Println(result)
			// Must be valid JSON
			var parsed interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestJSONValidation[%d] produced invalid JSON: %v\nOutput: %s", i, err, result)
			}

			// Must not be empty
			if result == "" {
				t.Errorf("TestJSONValidation[%d] produced empty output", i)
			}
		})
	}
}

// TestDumpMethodsJSON tests the various dump methods return consistent JSON
func TestDumpMethodsJSON(t *testing.T) {
	input := map[string]int{"test": 42}

	// ToJSON
	result1 := ToJSON(input)
	result1 = strings.TrimSpace(result1)

	// SdumpJSON (should be identical)
	result2 := SdumpJSON(input)
	result2 = strings.TrimSpace(result2)

	// ConfigState.ToJSON
	cs := ConfigState{Indent: " "}
	result3 := cs.ToJSON(input)
	result3 = strings.TrimSpace(result3)

	// All should produce valid JSON
	for i, result := range []string{result1, result2, result3} {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("TestDumpMethodsJSON[method_%d] produced invalid JSON: %v", i, err)
		}
	}

	// ToJSON and SdumpJSON should be identical
	if result1 != result2 {
		t.Errorf("ToJSON and SdumpJSON produced different results:\nToJSON:    %s\nSdumpJSON: %s", result1, result2)
	}
}

// TestMultiDimensionalJSON tests JSON output for multi-dimensional arrays and slices
func TestMultiDimensionalJSON(t *testing.T) {
	// 使用变量而不是常量
	var twoDimArray = [2][3]int{{1, 2, 3}, {4, 5, 6}}
	var sliceOfSlice = [][]string{{"a", "b"}, {"c", "d"}, {"e"}}
	var threeDimSlice = [][][]int{{{1, 2}, {3}}, {{4, 5, 6}}}

	tests := []struct {
		name  string
		input interface{}
	}{
		{"two_dim_array", twoDimArray},
		{"slice_of_slice", sliceOfSlice},
		{"three_dim_slice", threeDimSlice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestMultiDimensionalJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestNestedContainersJSON tests JSON output for nested container types
func TestNestedContainersJSON(t *testing.T) {
	// 使用变量而不是常量
	var mapOfSlice = map[string][]int{"numbers": {1, 2, 3}, "more": {4, 5}}
	var sliceOfMap = []map[string]int{{"a": 1, "b": 2}, {"c": 3}}
	var mapOfMap = map[string]map[string]int{"group1": {"x": 1, "y": 2}, "group2": {"z": 3}}
	var sliceOfStruct = []struct {
		Name  string
		Value int
	}{{"item1", 10}, {"item2", 20}}

	tests := []struct {
		name  string
		input interface{}
	}{
		{"map_of_slice", mapOfSlice},
		{"slice_of_map", sliceOfMap},
		{"map_of_map", mapOfMap},
		{"slice_of_struct", sliceOfStruct},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestNestedContainersJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestCustomTypesJSON tests JSON output for custom types and type aliases
func TestCustomTypesJSON(t *testing.T) {
	// 定义自定义类型
	type MyInt int
	type MyString string
	type MyFloat float64
	type MyBool bool

	// 使用变量而不是常量
	var myInt = MyInt(42)
	var myString = MyString("custom")
	var myFloat = MyFloat(3.14)
	var myBool = MyBool(true)

	// 基于slice的自定义类型
	type StringList []string
	var stringList = StringList{"hello", "world"}

	// 基于map的自定义类型
	type StringMap map[string]int
	var stringMap = StringMap{"a": 1, "b": 2}

	tests := []struct {
		name  string
		input interface{}
	}{
		{"custom_int", myInt},
		{"custom_string", myString},
		{"custom_float", myFloat},
		{"custom_bool", myBool},
		{"custom_slice", stringList},
		{"custom_map", stringMap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestCustomTypesJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestRuneAndStringSpecialJSON tests JSON output for rune and special strings
func TestRuneAndStringSpecialJSON(t *testing.T) {
	// 使用变量而不是常量
	var runeVar = 'A'
	var unicodeRune = '中'
	var emojiRune = '😀'
	var unicodeString = "Hello 世界 🌍"
	var specialCharsString = "Line1\nLine2\tTabbed\"Quoted\\"
	var emptyRune = rune(0)

	tests := []struct {
		name  string
		input interface{}
	}{
		{"ascii_rune", runeVar},
		{"unicode_rune", unicodeRune},
		{"emoji_rune", emojiRune},
		{"unicode_string", unicodeString},
		{"special_chars_string", specialCharsString},
		{"empty_rune", emptyRune},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestRuneAndStringSpecialJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestFloatSpecialValuesJSON tests JSON output for special float values
func TestFloatSpecialValuesJSON(t *testing.T) {
	// 使用变量而不是常量
	var nanFloat32 = float32(math.NaN())
	var nanFloat64 = math.NaN()
	var infFloat32 = float32(math.Inf(1))
	var infFloat64 = math.Inf(1)
	var negInfFloat32 = float32(math.Inf(-1))
	var negInfFloat64 = math.Inf(-1)
	var zeroFloat = 0.0
	var negZeroFloat = math.Copysign(0, -1)

	tests := []struct {
		name  string
		input interface{}
	}{
		{"nan_float32", nanFloat32},
		{"nan_float64", nanFloat64},
		{"inf_float32", infFloat32},
		{"inf_float64", infFloat64},
		{"neg_inf_float32", negInfFloat32},
		{"neg_inf_float64", negInfFloat64},
		{"zero_float", zeroFloat},
		{"neg_zero_float", negZeroFloat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// For special float values (NaN, Inf), JSON standard doesn't support them
			// so we just check that result is not empty and contains expected representation
			if strings.Contains(tt.name, "nan") {
				if !strings.Contains(result, "NaN") {
					t.Errorf("TestFloatSpecialValuesJSON[%s] expected NaN representation, got: %s", tt.name, result)
				}
			} else if strings.Contains(tt.name, "inf") {
				if !strings.Contains(result, "Inf") {
					t.Errorf("TestFloatSpecialValuesJSON[%s] expected Inf representation, got: %s", tt.name, result)
				}
			} else {
				// For normal float values, verify it's valid JSON
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					t.Errorf("TestFloatSpecialValuesJSON[%s] produced invalid JSON: %v", tt.name, err)
				}
			}
		})
	}
}

// TestStructAdvancedJSON tests JSON output for advanced struct scenarios
func TestStructAdvancedJSON(t *testing.T) {
	// 匿名字段结构体
	type Base struct {
		ID   int
		Name string
	}

	type Extended struct {
		Base
		Extra string
	}

	// 嵌套匿名结构体
	type WithAnonymous struct {
		PublicField  string
		privateField int // 私有字段
		Anonymous    struct {
			InnerField string
		}
	}

	// 使用变量而不是常量
	var extended = Extended{
		Base:  Base{ID: 1, Name: "test"},
		Extra: "additional",
	}

	var withAnonymous = WithAnonymous{
		PublicField:  "public",
		privateField: 42,
	}
	withAnonymous.Anonymous.InnerField = "inner"

	// 空指针字段的结构体
	type WithPointers struct {
		Name   string
		Value  *int
		Nested *Base
	}

	var val = 100
	var withPointers = WithPointers{
		Name:   "pointer test",
		Value:  &val,
		Nested: nil,
	}

	tests := []struct {
		name  string
		input interface{}
	}{
		{"struct_with_embedded", extended},
		{"struct_with_anonymous", withAnonymous},
		{"struct_with_pointers", withPointers},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestStructAdvancedJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestInterfaceAdvancedJSON tests JSON output for advanced interface scenarios
func TestInterfaceAdvancedJSON(t *testing.T) {
	// 使用变量而不是常量
	var structInterface interface{} = struct {
		Name string
		Age  int
	}{"John", 30}

	var sliceInterface interface{} = []int{1, 2, 3}
	var mapInterface interface{} = map[string]int{"key": 42}
	var pointerInterface interface{} = func() *int { i := 100; return &i }()

	// 接口的接口
	var nestedInterface interface{} = interface{}("nested")

	tests := []struct {
		name  string
		input interface{}
	}{
		{"interface_with_struct", structInterface},
		{"interface_with_slice", sliceInterface},
		{"interface_with_map", mapInterface},
		{"interface_with_pointer", pointerInterface},
		{"nested_interface", nestedInterface},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestInterfaceAdvancedJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestMapAdvancedKeysJSON tests JSON output for maps with different key types
func TestMapAdvancedKeysJSON(t *testing.T) {
	// 使用变量而不是常量
	var mapIntKey = map[int]string{1: "one", 2: "two", 100: "hundred"}
	var mapFloat64Key = map[float64]int{1.1: 11, 2.2: 22}
	var mapBoolKey = map[bool]string{true: "yes", false: "no"}

	// 结构体作为键（只有可比较的字段）
	type SimpleKey struct {
		X, Y int
	}

	var mapStructKey = map[SimpleKey]string{
		{1, 2}: "point1",
		{3, 4}: "point2",
	}

	tests := []struct {
		name  string
		input interface{}
	}{
		{"map_int_key", mapIntKey},
		{"map_float64_key", mapFloat64Key},
		{"map_bool_key", mapBoolKey},
		{"map_struct_key", mapStructKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestMapAdvancedKeysJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestDeepNestingJSON tests JSON output for deeply nested structures
func TestDeepNestingJSON(t *testing.T) {
	// 使用变量而不是常量
	var deepSlice = [][][][][]int{{{{{1, 2}}}}}

	type DeepNested struct {
		Level1 *struct {
			Level2 *struct {
				Level3 *struct {
					Value string
				}
			}
		}
	}

	var deepStruct = DeepNested{
		Level1: &struct {
			Level2 *struct {
				Level3 *struct {
					Value string
				}
			}
		}{
			Level2: &struct {
				Level3 *struct {
					Value string
				}
			}{
				Level3: &struct {
					Value string
				}{
					Value: "deep value",
				},
			},
		},
	}

	// 多级指针
	var value = 42
	var ptr1 = &value
	var ptr2 = &ptr1
	var ptr3 = &ptr2
	var ptr4 = &ptr3

	tests := []struct {
		name  string
		input interface{}
	}{
		{"deep_slice", deepSlice},
		{"deep_struct", deepStruct},
		{"multi_level_pointer", ptr4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestDeepNestingJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestEdgeCasesJSON tests JSON output for various edge cases
func TestEdgeCasesJSON(t *testing.T) {
	// 使用变量而不是常量
	var largeSlice = make([]int, 1000)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	var sparseSlice = make([]interface{}, 10)
	sparseSlice[0] = "first"
	sparseSlice[5] = 42
	sparseSlice[9] = "last"

	// 混合类型的slice（通过interface{}）
	var mixedSlice = []interface{}{
		1, "hello", true, 3.14, []int{1, 2}, map[string]int{"a": 1},
	}

	tests := []struct {
		name  string
		input interface{}
	}{
		{"large_slice", largeSlice},
		{"sparse_slice", sparseSlice},
		{"mixed_slice", mixedSlice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			result = strings.TrimSpace(result)

			// For large slice, just check it's not empty and valid JSON
			if tt.name == "large_slice" {
				if len(result) < 100 {
					t.Errorf("TestEdgeCasesJSON[%s] result too short: %d chars", tt.name, len(result))
				}
			} else {
				fmt.Println(result)
			}

			// Verify the result is valid JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestEdgeCasesJSON[%s] produced invalid JSON: %v", tt.name, err)
			}
		})
	}
}

// TestMaxDepthJSON tests the MaxDepth configuration for JSON output
func TestMaxDepthJSON(t *testing.T) {
	// 创建深度嵌套的结构体
	type Level struct {
		Value string
		Child *Level
	}

	// 创建5层深度的嵌套结构
	var deepStruct = &Level{
		Value: "Level1",
		Child: &Level{
			Value: "Level2",
			Child: &Level{
				Value: "Level3",
				Child: &Level{
					Value: "Level4",
					Child: &Level{
						Value: "Level5",
						Child: nil,
					},
				},
			},
		},
	}

	// 创建深度嵌套的slice
	var deepSlice = []interface{}{
		"Level1",
		[]interface{}{
			"Level2",
			[]interface{}{
				"Level3",
				[]interface{}{
					"Level4",
					[]interface{}{
						"Level5",
					},
				},
			},
		},
	}

	// 创建深度嵌套的map
	var deepMap = map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"level4": map[string]interface{}{
						"level5": "deep value",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		input    interface{}
		maxDepth int
		checkFn  func(t *testing.T, result string, maxDepth int)
	}{
		{
			name:     "struct_max_depth_1",
			input:    deepStruct,
			maxDepth: 1,
			checkFn:  validateMaxDepthStruct,
		},
		{
			name:     "struct_max_depth_3",
			input:    deepStruct,
			maxDepth: 3,
			checkFn:  validateMaxDepthStruct,
		},
		{
			name:     "struct_max_depth_unlimited",
			input:    deepStruct,
			maxDepth: 0, // 0 means unlimited
			checkFn:  validateMaxDepthStruct,
		},
		{
			name:     "slice_max_depth_1",
			input:    deepSlice,
			maxDepth: 1,
			checkFn:  validateMaxDepthSlice,
		},
		{
			name:     "slice_max_depth_2",
			input:    deepSlice,
			maxDepth: 2,
			checkFn:  validateMaxDepthSlice,
		},
		{
			name:     "slice_max_depth_unlimited",
			input:    deepSlice,
			maxDepth: 0,
			checkFn:  validateMaxDepthSlice,
		},
		{
			name:     "map_max_depth_1",
			input:    deepMap,
			maxDepth: 1,
			checkFn:  validateMaxDepthMap,
		},
		{
			name:     "map_max_depth_2",
			input:    deepMap,
			maxDepth: 2,
			checkFn:  validateMaxDepthMap,
		},
		{
			name:     "map_max_depth_unlimited",
			input:    deepMap,
			maxDepth: 0,
			checkFn:  validateMaxDepthMap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := ConfigState{
				MaxDepth: tt.maxDepth,
			}
			result := cs.ToJSON(tt.input)
			result = strings.TrimSpace(result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestMaxDepthJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// 调用特定的验证函数
			if tt.checkFn != nil {
				tt.checkFn(t, result, tt.maxDepth)
			}

			fmt.Printf("=== %s (MaxDepth: %d) ===\n%s\n\n", tt.name, tt.maxDepth, result)
		})
	}
}

// validateMaxDepthStruct 验证结构体的最大深度限制
func validateMaxDepthStruct(t *testing.T, result string, maxDepth int) {
	// 解析JSON结果
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("validateMaxDepthStruct: invalid JSON: %v", err)
		return
	}

	// 检查是否被截断
	if maxDepth > 0 {
		// 对于有限制的深度，检查是否包含截断指示
		depthCount := countNestedDepthInJSON(result, "Child")
		if depthCount > maxDepth {
			t.Errorf("validateMaxDepthStruct: expected max depth %d, but found depth %d", maxDepth, depthCount)
		}
	} else {
		// 对于无限制深度，应该能看到所有层级
		if !strings.Contains(result, "Level5") {
			t.Errorf("validateMaxDepthStruct: unlimited depth should contain Level5")
		}
	}
}

// validateMaxDepthSlice 验证slice的最大深度限制
func validateMaxDepthSlice(t *testing.T, result string, maxDepth int) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("validateMaxDepthSlice: invalid JSON: %v", err)
		return
	}

	if maxDepth > 0 {
		// 检查嵌套数组的深度
		depthCount := countArrayNestingInJSON(result)
		if depthCount > maxDepth {
			t.Errorf("validateMaxDepthSlice: expected max depth %d, but found depth %d", maxDepth, depthCount)
		}
	} else {
		// 无限制深度应该包含最深层的值
		if !strings.Contains(result, "Level5") {
			t.Errorf("validateMaxDepthSlice: unlimited depth should contain Level5")
		}
	}
}

// validateMaxDepthMap 验证map的最大深度限制
func validateMaxDepthMap(t *testing.T, result string, maxDepth int) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("validateMaxDepthMap: invalid JSON: %v", err)
		return
	}

	if maxDepth > 0 {
		// 对于有限制的深度，检查嵌套层数
		if maxDepth >= 5 || maxDepth == 0 {
			// 足够深或无限制，应该能看到最深的值
			if !strings.Contains(result, "deep value") {
				t.Errorf("validateMaxDepthMap: should contain 'deep value' with depth %d", maxDepth)
			}
		}
	} else {
		// 无限制深度
		if !strings.Contains(result, "deep value") {
			t.Errorf("validateMaxDepthMap: unlimited depth should contain 'deep value'")
		}
	}
}

// countNestedDepthInJSON 计算JSON中特定字段的嵌套深度
func countNestedDepthInJSON(jsonStr, fieldName string) int {
	count := 0
	searchStr := fmt.Sprintf(`"%s"`, fieldName)

	for strings.Contains(jsonStr, searchStr) {
		count++
		// 移除第一个匹配，继续查找下一个嵌套层
		index := strings.Index(jsonStr, searchStr)
		if index == -1 {
			break
		}
		jsonStr = jsonStr[index+len(searchStr):]
	}

	return count
}

// countArrayNestingInJSON 计算JSON中数组的嵌套深度
func countArrayNestingInJSON(jsonStr string) int {
	maxDepth := 0
	currentDepth := 0

	for _, char := range jsonStr {
		switch char {
		case '[':
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		case ']':
			currentDepth--
		}
	}

	return maxDepth
}

// TestMaxDepthCircularReferenceJSON 测试MaxDepth在循环引用中的行为
func TestMaxDepthCircularReferenceJSON(t *testing.T) {
	type Node struct {
		Name string
		Next *Node
	}

	// 创建循环引用
	node1 := &Node{Name: "Node1"}
	node2 := &Node{Name: "Node2"}
	node1.Next = node2
	node2.Next = node1 // 创建循环

	tests := []struct {
		name     string
		maxDepth int
	}{
		{"circular_max_depth_1", 1},
		{"circular_max_depth_3", 3},
		{"circular_max_depth_5", 5},
		{"circular_unlimited", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := ConfigState{
				MaxDepth: tt.maxDepth,
			}

			// 这不应该导致无限循环或崩溃
			result := cs.ToJSON(node1)
			result = strings.TrimSpace(result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestMaxDepthCircularReferenceJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// 结果不应该为空
			if result == "" {
				t.Errorf("TestMaxDepthCircularReferenceJSON[%s] produced empty result", tt.name)
			}

			fmt.Printf("=== %s ===\n%s\n\n", tt.name, result)
		})
	}
}

// TestMaxDepthComplexStructuresJSON 测试复杂数据结构的MaxDepth
func TestMaxDepthComplexStructuresJSON(t *testing.T) {
	// 复杂的混合嵌套结构
	type ComplexStruct struct {
		Name     string
		Children []interface{}
		MetaData map[string]interface{}
		Parent   *ComplexStruct
	}

	// 创建复杂的嵌套结构
	var root = &ComplexStruct{
		Name: "Root",
		Children: []interface{}{
			"child1",
			map[string]interface{}{
				"nested": []interface{}{
					"deep1",
					map[string]interface{}{
						"deeper": "value",
					},
				},
			},
		},
		MetaData: map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": "meta value",
				},
			},
		},
	}

	// 添加一个子结构
	child := &ComplexStruct{
		Name:     "Child",
		Children: []interface{}{"grandchild"},
		MetaData: map[string]interface{}{"type": "child"},
		Parent:   root,
	}
	root.Children = append(root.Children, child)

	tests := []struct {
		name     string
		maxDepth int
	}{
		{"complex_max_depth_1", 1},
		{"complex_max_depth_2", 2},
		{"complex_max_depth_4", 4},
		{"complex_unlimited", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := ConfigState{
				MaxDepth: tt.maxDepth,
			}

			result := cs.ToJSON(root)
			result = strings.TrimSpace(result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestMaxDepthComplexStructuresJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// 检查深度限制是否生效
			if tt.maxDepth > 0 && tt.maxDepth < 4 {
				// 对于较浅的深度，不应该包含很深的值
				if strings.Contains(result, "meta value") && tt.maxDepth < 3 {
					t.Errorf("TestMaxDepthComplexStructuresJSON[%s] should not contain deep values with maxDepth %d", tt.name, tt.maxDepth)
				}
			}

			fmt.Printf("=== %s ===\n%s\n\n", tt.name, result)
		})
	}
}

// TestMaxDepthPerformanceJSON 测试MaxDepth对性能的影响
func TestMaxDepthPerformanceJSON(t *testing.T) {
	// 创建一个非常大的深度结构用于性能测试
	type DeepStruct struct {
		Level int
		Child *DeepStruct
	}

	// 创建20层深度的结构
	var buildDeepStruct func(level, maxLevel int) *DeepStruct
	buildDeepStruct = func(level, maxLevel int) *DeepStruct {
		if level >= maxLevel {
			return nil
		}
		return &DeepStruct{
			Level: level,
			Child: buildDeepStruct(level+1, maxLevel),
		}
	}

	deepStruct := buildDeepStruct(0, 20)

	tests := []struct {
		name     string
		maxDepth int
	}{
		{"performance_max_depth_5", 5},
		{"performance_max_depth_10", 10},
		{"performance_unlimited", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := ConfigState{
				MaxDepth: tt.maxDepth,
			}

			// 测量执行时间
			start := time.Now()
			result := cs.ToJSON(deepStruct)
			duration := time.Since(start)

			result = strings.TrimSpace(result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestMaxDepthPerformanceJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			fmt.Printf("=== %s (Duration: %v) ===\nResult length: %d characters\n\n",
				tt.name, duration, len(result))

			// 对于有限制的深度，执行应该相对较快
			if tt.maxDepth > 0 && duration > time.Second {
				t.Logf("TestMaxDepthPerformanceJSON[%s] took %v, consider if this is acceptable", tt.name, duration)
			}
		})
	}
}

// TestImprovedMaxDepthJSON 测试改进的MaxDepth显示功能
func TestImprovedMaxDepthJSON(t *testing.T) {
	// 创建深度嵌套的结构体用于测试
	type NestedStruct struct {
		Name    string
		Level   int
		Child   *NestedStruct
		Data    []int
		Mapping map[string]interface{}
	}

	// 创建包含多种类型的深度结构
	var deepStruct = &NestedStruct{
		Name:  "Level1",
		Level: 1,
		Data:  []int{1, 2, 3, 4, 5},
		Mapping: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
		Child: &NestedStruct{
			Name:  "Level2",
			Level: 2,
			Data:  []int{10, 20, 30},
			Mapping: map[string]interface{}{
				"nested": map[string]interface{}{
					"deep": "value",
				},
			},
			Child: &NestedStruct{
				Name:    "Level3",
				Level:   3,
				Data:    []int{100, 200},
				Mapping: map[string]interface{}{"final": "data"},
				Child:   nil,
			},
		},
	}

	tests := []struct {
		name     string
		input    interface{}
		maxDepth int
		checkFn  func(t *testing.T, result string)
	}{
		{
			name:     "improved_struct_depth_1",
			input:    deepStruct,
			maxDepth: 1,
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected truncation info with __truncated__ field")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type information")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth information")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth information")
				}
			},
		},
		{
			name:     "improved_slice_depth_1",
			input:    []interface{}{[]interface{}{[]interface{}{"deep"}}},
			maxDepth: 1,
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected truncation info for slice")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type information for slice")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth information for slice")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth information for slice")
				}
			},
		},
		{
			name: "improved_map_depth_1",
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": "deep value",
				},
			},
			maxDepth: 1,
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected truncation info for map")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type information for map")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth information for map")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth information for map")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := ConfigState{
				MaxDepth: tt.maxDepth,
			}
			result := cs.ToJSON(tt.input)
			result = strings.TrimSpace(result)

			fmt.Printf("=== %s ===\n%s\n\n", tt.name, result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestImprovedMaxDepthJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// 运行特定的检查函数
			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

// TestDetailedTruncationInfoJSON 测试详细的截断信息
func TestDetailedTruncationInfoJSON(t *testing.T) {
	// 测试结构体的详细信息
	type LargeStruct struct {
		Field1  string
		Field2  int
		Field3  bool
		Field4  float64
		Field5  []int
		Field6  map[string]string
		Field7  *string
		Private int
	}

	var str = "test"
	var largeStruct = LargeStruct{
		Field1:  "value1",
		Field2:  42,
		Field3:  true,
		Field4:  3.14,
		Field5:  []int{1, 2, 3},
		Field6:  map[string]string{"key": "value"},
		Field7:  &str,
		Private: 100,
	}

	// 测试大数组的详细信息
	var largeArray = make([]int, 100)
	for i := range largeArray {
		largeArray[i] = i
	}

	// 测试大map的详细信息
	var largeMap = make(map[string]int)
	for i := 0; i < 50; i++ {
		largeMap[fmt.Sprintf("key%d", i)] = i
	}

	tests := []struct {
		name    string
		input   interface{}
		checkFn func(t *testing.T, result string)
	}{
		{
			name:  "large_struct_truncation",
			input: map[string]interface{}{"data": largeStruct},
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected __truncated__ for struct truncation")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type for struct truncation")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth for struct truncation")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth for struct truncation")
				}
			},
		},
		{
			name:  "large_array_truncation",
			input: []interface{}{largeArray},
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected __truncated__ for array truncation")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type for array truncation")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth for array truncation")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth for array truncation")
				}
			},
		},
		{
			name:  "large_map_truncation",
			input: []interface{}{largeMap},
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected __truncated__ for map truncation")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type for map truncation")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth for map truncation")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth for map truncation")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := ConfigState{
				MaxDepth: 1, // 强制截断
			}
			result := cs.ToJSON(tt.input)
			result = strings.TrimSpace(result)

			fmt.Printf("=== %s ===\n%s\n\n", tt.name, result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestDetailedTruncationInfoJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// 运行特定的检查函数
			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

// TestPointerAndInterfaceTruncationJSON 测试指针和接口的截断信息
func TestPointerAndInterfaceTruncationJSON(t *testing.T) {
	// 创建指针链
	var value = 42
	var ptr1 = &value
	var ptr2 = &ptr1

	// 创建接口包装
	var interfaceValue interface{} = map[string]interface{}{
		"nested": "value",
	}

	tests := []struct {
		name    string
		input   interface{}
		checkFn func(t *testing.T, result string)
	}{
		{
			name:  "pointer_truncation",
			input: []interface{}{ptr2},
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected __truncated__ for pointer truncation")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type for pointer truncation")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth for pointer truncation")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth for pointer truncation")
				}
			},
		},
		{
			name:  "interface_truncation",
			input: []interface{}{interfaceValue},
			checkFn: func(t *testing.T, result string) {
				if !strings.Contains(result, "__truncated__") {
					t.Error("Expected __truncated__ for interface truncation")
				}
				if !strings.Contains(result, "type") {
					t.Error("Expected type for interface truncation")
				}
				if !strings.Contains(result, "depth") {
					t.Error("Expected depth for interface truncation")
				}
				if !strings.Contains(result, "max_depth") {
					t.Error("Expected max_depth for interface truncation")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := ConfigState{
				MaxDepth: 1, // 强制截断
			}
			result := cs.ToJSON(tt.input)
			result = strings.TrimSpace(result)

			fmt.Printf("=== %s ===\n%s\n\n", tt.name, result)

			// 验证结果是有效的JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("TestPointerAndInterfaceTruncationJSON[%s] produced invalid JSON: %v", tt.name, err)
				return
			}

			// 运行特定的检查函数
			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

// TestCompareOldVsNewMaxDepthJSON 比较旧版本和新版本的MaxDepth输出
func TestCompareOldVsNewMaxDepthJSON(t *testing.T) {
	// 创建测试数据
	type TestData struct {
		Name  string
		Items []interface{}
		Meta  map[string]interface{}
	}

	var testData = TestData{
		Name: "TestData",
		Items: []interface{}{
			"item1",
			map[string]interface{}{
				"nested": "value",
			},
		},
		Meta: map[string]interface{}{
			"version": "1.0",
			"config":  map[string]interface{}{"debug": true},
		},
	}

	// 测试在深度1的情况下的输出
	cs := ConfigState{
		MaxDepth: 1,
	}

	result := cs.ToJSON(testData)
	result = strings.TrimSpace(result)

	fmt.Printf("=== 改进后的MaxDepth输出 ===\n%s\n\n", result)

	// 验证新的输出包含更多有用信息
	t.Run("improved_output_validation", func(t *testing.T) {
		// 验证结果是有效的JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("Improved MaxDepth output produced invalid JSON: %v", err)
			return
		}

		// 检查是否包含改进的信息
		hasImprovedInfo := strings.Contains(result, "__truncated__") ||
			strings.Contains(result, "num_fields") ||
			strings.Contains(result, "length") ||
			strings.Contains(result, "type")

		if !hasImprovedInfo {
			t.Error("Expected improved truncation information in output")
		}

		// 确保不再是简单的 "max depth reached" 字符串
		if strings.Contains(result, `"max depth reached"`) && !strings.Contains(result, "__truncated__") {
			t.Error("Output still uses old simple 'max depth reached' format")
		}
	})
}

// 用于unsafe dump测试的结构体
// 包含导出和未导出字段

type testUnsafeStruct struct {
	Exported   int
	unexported string
}

func TestUnsafeDump_UnexportedField(t *testing.T) {
	obj := testUnsafeStruct{
		Exported:   42,
		unexported: "hidden",
	}
	jsonStr := SdumpJSON(&obj)
	if !strings.Contains(jsonStr, "hidden") {
		t.Errorf("unsafe dump 未能导出未导出字段，输出: %s", jsonStr)
	}

	if !strings.Contains(jsonStr, "Exported") || !strings.Contains(jsonStr, "unexported") {
		t.Errorf("字段名未包含在输出中: %s", jsonStr)
	}
	fmt.Println(jsonStr)

	// go-spew 对比测试
	goSpewStr := gspew.Sdump(&obj)
	if !strings.Contains(goSpewStr, "hidden") {
		t.Errorf("go-spew 未能导出未导出字段，输出: %s", goSpewStr)
	}
	fmt.Println(goSpewStr)
	if !strings.Contains(goSpewStr, "Exported") || !strings.Contains(goSpewStr, "unexported") {
		t.Errorf("go-spew 字段名未包含在输出中: %s", goSpewStr)
	}
	fmt.Println(goSpewStr)
}

func TestSdump_ComplexStructWithUnsupportedFields(t *testing.T) {
	type dummyStream struct{}
	type dummyConn struct{}
	type grpcTunnel struct {
		stream   interface{}
		sendLock *sync.Mutex
		recvLock *sync.Mutex
		grpcConn *dummyConn
		pending  chan struct{}
		fn       func()
	}

	tunnel := &grpcTunnel{
		stream:   &dummyStream{},
		sendLock: &sync.Mutex{},
		recvLock: &sync.Mutex{},
		grpcConn: &dummyConn{},
		pending:  make(chan struct{}),
		fn:       func() {},
	}

	output := Sdump(tunnel)
	if !strings.Contains(output, "<ptr") || !strings.Contains(output, "<chan") || !strings.Contains(output, "<func") {
		t.Errorf("Sdump output should contain type/address placeholders for unsupported fields, got: %s", output)
	}
	fmt.Println(output)
	fmt.Println(gspew.Sdump(tunnel))
}
