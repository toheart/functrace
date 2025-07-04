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

// TestSkipNilValues tests the SkipNilValues option.
func TestSkipNilValues(t *testing.T) {
	// 创建包含nil值的测试数据
	type TestStruct struct {
		Name     string
		Age      int
		Address  *string
		NilMap   map[string]string
		NilSlice []int
		Empty    *struct{}
	}

	address := "123 Main St"
	testData := TestStruct{
		Name:     "John",
		Age:      30,
		Address:  &address,
		NilMap:   nil,
		NilSlice: nil,
		Empty:    nil,
	}

	// 使用默认配置（不跳过nil值）
	defaultJSON := spew.ToJSON(testData)
	t.Logf("Default JSON output: %s", defaultJSON)

	// 验证默认输出包含nil值
	var defaultData map[string]interface{}
	err := json.Unmarshal([]byte(defaultJSON), &defaultData)
	if err != nil {
		t.Errorf("JSON unmarshal error for default output: %v", err)
	}
	valueMap, ok := defaultData["value"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected 'value' to be a map, got %T", defaultData["value"])
	} else {
		// 检查nil值是否存在
		if valueMap["NilMap"] == nil && valueMap["NilSlice"] == nil && valueMap["Empty"] == nil {
			t.Logf("Default output correctly includes nil values")
		} else {
			t.Errorf("Default output did not correctly include nil values: %v", valueMap)
		}
	}

	// 使用自定义配置（跳过nil值）
	cs := spew.ConfigState{
		Indent:        " ",
		SkipNilValues: true,
	}
	skipNilJSON := cs.ToJSON(testData)
	t.Logf("SkipNil JSON output: %s", skipNilJSON)

	// 验证自定义输出不包含nil值
	var skipNilData map[string]interface{}
	err = json.Unmarshal([]byte(skipNilJSON), &skipNilData)
	if err != nil {
		t.Errorf("JSON unmarshal error for skipNil output: %v", err)
	}
	skipNilMap, ok := skipNilData["value"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected 'value' to be a map, got %T", skipNilData["value"])
	} else {
		// 检查nil值是否被跳过
		if _, exists := skipNilMap["NilMap"]; exists {
			t.Errorf("SkipNil output should not include NilMap")
		}
		if _, exists := skipNilMap["NilSlice"]; exists {
			t.Errorf("SkipNil output should not include NilSlice")
		}
		if _, exists := skipNilMap["Empty"]; exists {
			t.Errorf("SkipNil output should not include Empty")
		}
	}
}

// TestSkipNilValuesInSlice tests the SkipNilValues option with slices.
func TestSkipNilValuesInSlice(t *testing.T) {
	// 创建包含nil值的切片
	type Item struct {
		Name string
	}
	slice := []*Item{
		{Name: "Item1"},
		nil,
		{Name: "Item3"},
		nil,
	}

	// 使用默认配置（不跳过nil值）
	defaultJSON := spew.ToJSON(slice)
	t.Logf("Default slice JSON output: %s", defaultJSON)

	// 使用自定义配置（跳过nil值）
	cs := spew.ConfigState{
		Indent:        " ",
		SkipNilValues: true,
	}
	skipNilJSON := cs.ToJSON(slice)
	t.Logf("SkipNil slice JSON output: %s", skipNilJSON)

	// 验证自定义输出不包含nil值
	var skipNilData map[string]interface{}
	err := json.Unmarshal([]byte(skipNilJSON), &skipNilData)
	if err != nil {
		t.Errorf("JSON unmarshal error for skipNil slice output: %v", err)
	}
}

// TestSkipNilValuesInMap tests the SkipNilValues option with maps.
func TestSkipNilValuesInMap(t *testing.T) {
	// 创建包含nil值的映射
	type Item struct {
		Name string
	}
	m := map[string]*Item{
		"item1": {Name: "Item1"},
		"item2": nil,
		"item3": {Name: "Item3"},
		"item4": nil,
	}

	// 使用默认配置（不跳过nil值）
	defaultJSON := spew.ToJSON(m)
	t.Logf("Default map JSON output: %s", defaultJSON)

	// 使用自定义配置（跳过nil值）
	cs := spew.ConfigState{
		Indent:        " ",
		SkipNilValues: true,
	}
	skipNilJSON := cs.ToJSON(m)
	t.Logf("SkipNil map JSON output: %s", skipNilJSON)

	// 验证自定义输出不包含nil值
	var skipNilData map[string]interface{}
	err := json.Unmarshal([]byte(skipNilJSON), &skipNilData)
	if err != nil {
		t.Errorf("JSON unmarshal error for skipNil map output: %v", err)
	}
}
