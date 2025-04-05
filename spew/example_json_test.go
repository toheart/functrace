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
	"fmt"

	"github.com/toheart/functrace/spew"
)

// 这个简单示例展示了如何使用ToJSON函数
func ExampleToJSON_simple() {
	// 一个简单的整数值
	value := 123

	// 输出JSON格式
	jsonOutput := spew.ToJSON(value)
	fmt.Println(jsonOutput)
	// Output: {"value":123}
}

// 这个示例展示了如何使用ConfigState定制JSON输出
func ExampleConfigState_ToJSON_simple() {
	// 创建简单数据结构
	type Simple struct {
		Name string
		Age  int
	}

	// 创建一个实例
	data := Simple{
		Name: "张三",
		Age:  30,
	}

	// 创建自定义配置
	cs := spew.ConfigState{
		Indent:           "  ",
		EnableJSONOutput: true,
		SkipNilValues:    true,
	}

	// 获取并输出JSON字符串
	jsonOutput := cs.ToJSON(data)
	fmt.Println(jsonOutput)

	// Output: {"value":{"type":"spew_test.Simple","Name":"张三","Age":30}}
}

// 这个示例展示了如何使用SdumpJSON输出JSON字符串
func ExampleSdumpJSON_simple() {
	// 简单结构
	type Person struct {
		Name string
		Age  int
	}

	// 创建一个Person实例
	person := Person{
		Name: "李四",
		Age:  25,
	}

	// 输出JSON格式
	fmt.Println(spew.SdumpJSON(person))
	// Output: {"value":{"type":"spew_test.Person","Name":"李四","Age":25}}
}
