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

package spew_test

import (
	"encoding/json"
	"testing"

	"github.com/toheart/functrace/spew"
)

// TestToJSON tests the ToJSON and SdumpJSON functions.
func TestToJSON(t *testing.T) {
	// 测试基本类型
	intValue := 123
	jsonStr := spew.ToJSON(intValue)
	t.Logf("JSON output for int: %s", jsonStr)

	// 验证输出是有效的JSON
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		t.Errorf("JSON unmarshal error: %v", err)
	}

	// 验证数据内容
	if val, ok := data["value"].(float64); !ok || int(val) != intValue {
		t.Errorf("Expected value to be %d, got %v", intValue, data["value"])
	}

	// 测试SdumpJSON函数
	jsonStr2 := spew.SdumpJSON(intValue)
	if jsonStr != jsonStr2 {
		t.Errorf("SdumpJSON should return the same output as ToJSON")
	}
}

// TestToJSONWithComplex tests the ToJSON function with complex structures.
func TestToJSONWithComplex(t *testing.T) {
	// 创建一个复杂的数据结构
	type Person struct {
		Name    string
		Age     int
		Address struct {
			Street string
			City   string
		}
		Friends []string
	}

	p := Person{
		Name: "John",
		Age:  30,
	}
	p.Address.Street = "123 Main St"
	p.Address.City = "New York"
	p.Friends = []string{"Alice", "Bob", "Charlie"}

	// 生成JSON输出
	jsonStr := spew.ToJSON(p)
	t.Logf("JSON output for complex struct: %s", jsonStr)

	// 验证输出是有效的JSON
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		t.Errorf("JSON unmarshal error: %v", err)
	}
}

// TestToJSONWithUint8 tests the ToJSON function with uint8 slices.
func TestToJSONWithUint8(t *testing.T) {
	// 测试包含可打印ASCII字符的uint8切片
	printable := []byte("Hello, World!")
	jsonStr := spew.ToJSON(printable)
	t.Logf("JSON output for printable uint8 slice: %s", jsonStr)

	// 验证输出是有效的JSON
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		t.Errorf("JSON unmarshal error: %v", err)
	}

	// 测试包含非ASCII字符的uint8切片
	nonPrintable := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	jsonStr = spew.ToJSON(nonPrintable)
	t.Logf("JSON output for non-printable uint8 slice: %s", jsonStr)

	// 验证输出是有效的JSON
	err = json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		t.Errorf("JSON unmarshal error: %v", err)
	}
}

// TestConfigStateToJSON tests the ConfigState.ToJSON method.
func TestConfigStateToJSON(t *testing.T) {
	cs := spew.ConfigState{
		Indent:                  "  ",
		MaxDepth:                5,
		DisableMethods:          true,
		DisablePointerAddresses: true,
		EnableJSONOutput:        false, // 这里设置为false，但在ToJSON中会暂时启用
	}

	// 测试基本类型
	intValue := 123
	jsonStr := cs.ToJSON(intValue)
	t.Logf("ConfigState.ToJSON output: %s", jsonStr)

	// 验证输出是有效的JSON
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		t.Errorf("JSON unmarshal error: %v", err)
	}

	// 验证EnableJSONOutput值没有被永久修改
	if cs.EnableJSONOutput != false {
		t.Errorf("EnableJSONOutput should remain false after ToJSON call")
	}
}
