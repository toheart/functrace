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

package spew

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"regexp"
	"sync"
	"unicode/utf8"
	"unsafe"
)

var (
	// uint8Type是表示uint8的reflect.Type。用于将cgo类型转换为uint8切片进行十六进制转储。
	uint8Type = reflect.TypeOf(uint8(0))

	// cCharRE是匹配cgo char的正则表达式。用于检测字符数组以进行十六进制转储。
	cCharRE = regexp.MustCompile(`^.*\._Ctype_char$`)

	// cUnsignedCharRE是匹配cgo unsigned char的正则表达式。用于检测无符号字符数组以进行十六进制转储。
	cUnsignedCharRE = regexp.MustCompile(`^.*\._Ctype_unsignedchar$`)

	// cUint8tCharRE是匹配cgo uint8_t的正则表达式。用于检测uint8_t数组以进行十六进制转储。
	cUint8tCharRE = regexp.MustCompile(`^.*\._Ctype_uint8_t$`)

	// dumpStatePool 是dumpState对象的内存池，用于复用对象以减少内存分配
	dumpStatePool = sync.Pool{
		New: func() interface{} {
			return &dumpState{
				pointers: make(map[uintptr]int),
			}
		},
	}
)

// dumpState 包含JSON转储操作的状态信息
// 这是纯JSON架构，不再支持文本输出
type dumpState struct {
	depth    int             // 当前递归深度
	pointers map[uintptr]int // 已处理的指针映射，用于检测循环引用
	cs       *ConfigState    // 配置状态
	root     interface{}     // 最终递归生成的对象树（map/slice/值）
	visited  map[uintptr]int // 用于检测循环引用
}

// reset 重置dumpState到初始状态，用于内存池复用
// 清理所有状态字段，准备下次使用
func (d *dumpState) reset() {
	d.depth = 0
	// 清空pointers map但保留其容量以避免重新分配
	for k := range d.pointers {
		delete(d.pointers, k)
	}
	d.cs = nil
	d.root = nil
	d.visited = nil
}

// getDumpState 从内存池获取一个dumpState对象并初始化
// 参数:
//   - cs: 配置状态
//
// 返回值:
//   - *dumpState: 初始化的dumpState对象
func getDumpState(cs *ConfigState) *dumpState {
	d := dumpStatePool.Get().(*dumpState)
	d.cs = cs
	d.depth = 0
	d.root = nil
	return d
}

// putDumpState 将dumpState对象归还到内存池
// 参数:
//   - d: 要归还的dumpState对象
func putDumpState(d *dumpState) {
	d.reset()
	dumpStatePool.Put(d)
}

// safeReflectCall 安全地执行反射操作，捕获可能的panic
// 参数:
//   - fn: 要执行的反射操作函数
//   - errorMsg: 发生panic时返回的错误消息
//
// 返回值:
//   - result: 函数执行结果或错误消息
//   - success: 操作是否成功
func (d *dumpState) safeReflectCall(fn func() interface{}, errorMsg string) (result interface{}, success bool) {
	defer func() {
		if r := recover(); r != nil {
			result = errorMsg
			success = false
		}
	}()

	result = fn()
	success = true
	return
}

// getUint8String 获取uint8数组或切片的字符串表示
// 参数:
//   - buf: uint8数组或切片
//
// 返回值:
//   - string: 十六进制格式的字符串表示
//
// 功能: 将字节数组转换为十六进制字符串，每行显示16个字节，最多显示2048个字节
func getUint8String(buf []uint8) string {
	if len(buf) == 0 {
		return ""
	}
	if len(buf) > 2048 {
		buf = buf[:2048]
	}

	// 检查是否为可打印的ASCII文本
	isPrintableASCII := true
	for _, b := range buf {
		// ASCII范围内的可打印字符（32-126）以及常见的空白字符（9, 10, 13）
		if !((b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13) {
			isPrintableASCII = false
			break
		}
	}

	// 如果是可打印的ASCII文本，直接返回字符串
	if isPrintableASCII {
		return string(buf)
	}

	// 否则返回十六进制表示
	var sb bytes.Buffer

	// 以十六进制格式化，每16个字节一行
	rows := len(buf) / 16
	if len(buf)%16 != 0 {
		rows++
	}

	sb.WriteString("[")
	for i := 0; i < rows; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}

		start := i * 16
		end := start + 16
		if end > len(buf) {
			end = len(buf)
		}

		sb.WriteString("\"")
		for j := start; j < end; j++ {
			fmt.Fprintf(&sb, "%02x", buf[j])
			if j < end-1 && (j-start+1)%2 == 0 {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\"")
	}
	sb.WriteString("]")

	return sb.String()
}

// unpackValue 在可能的情况下返回非nil接口内部的值。
// 参数:
//   - v: 要解包的reflect.Value
//
// 返回值:
//   - reflect.Value: 解包后的值
//
// 功能: 对于包含不同类型的接口（如structs、arrays、slices、maps），提取其实际值
func (d *dumpState) unpackValue(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	return v
}

// dump 是转储值的主要工作函数。它使用传入的反射值来确定我们正在处理什么类型的对象，并适当地格式化它。
// 这是一个递归函数，但是会检测并正确处理循环数据结构。
// 参数:
//   - v: 要转储的reflect.Value
//
// 功能: 根据值的类型进行相应的格式化输出，支持各种Go类型，包括基本类型、复合类型、指针等
func (d *dumpState) dump(v reflect.Value) interface{} {
	return d.dumpWithDepth(v, -1)
}

func (d *dumpState) dumpWithDepth(v reflect.Value, depth int) interface{} {
	if !v.IsValid() {
		return "invalid"
	}

	// 深度截断：只在进入子元素/字段时判断
	if d.cs.MaxDepth > 0 && depth > d.cs.MaxDepth && isDeepType(v.Kind()) {
		var l, c int
		switch v.Kind() {
		case reflect.Slice, reflect.Array:
			l = v.Len()
			c = v.Cap()
		case reflect.Map:
			l = v.Len()
			c = 0
		case reflect.Interface:
			l, c = 1, 1
		}
		return truncatedContainerInfo(v.Type().String(), depth, d.cs.MaxDepth, l, c)
	}

	if v.Kind() == reflect.Interface && !v.IsNil() {
		return d.dumpWithDepth(v.Elem(), depth+1)
	}

	var addr uintptr
	isRef := false
	switch v.Kind() {
	case reflect.Ptr:
		if v.CanAddr() {
			addr = v.UnsafeAddr()
			isRef = true
		}
	}
	if isRef && addr != 0 {
		if d.visited == nil {
			d.visited = make(map[uintptr]int)
		}
		if _, ok := d.visited[addr]; ok {
			return fmt.Sprintf("<circular %s %#x>", v.Type().String(), addr)
		}
		d.visited[addr] = depth
		defer delete(d.visited, addr)
	}

	var result interface{}
	switch v.Kind() {
	case reflect.Bool:
		result = v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result = v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		result = v.Uint()
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsNaN(f) {
			result = "NaN"
		} else if math.IsInf(f, 1) {
			result = "Inf"
		} else if math.IsInf(f, -1) {
			result = "-Inf"
		} else {
			result = f
		}
	case reflect.Complex64, reflect.Complex128:
		result = fmt.Sprintf("%v", v.Complex())
	case reflect.String:
		result = v.String()
	case reflect.Ptr:
		if v.IsNil() {
			result = nil
		} else {
			addr := v.Pointer()
			if d.visited != nil {
				if _, ok := d.visited[addr]; ok {
					result = fmt.Sprintf("<ptr %s %#x>", v.Type().String(), addr)
					break
				}
			}
			// 递归展开指向内容
			result = d.dumpWithDepth(v.Elem(), depth+1)
		}
	case reflect.Struct:
		result = dumpStructFields(d, v, depth)
	case reflect.Map:
		if v.IsNil() {
			result = nil
		} else {
			result = dumpMapFields(d, v, depth)
		}
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			b := v.Bytes()
			// 优化：省略末尾零填充
			totalLen := len(b)
			end := totalLen
			for end > 0 && b[end-1] == 0 {
				end--
			}
			zeroCount := totalLen - end
			var outStr string
			if isPrintableOrControlASCII(b[:end]) || isJSON(b[:end]) {
				outStr = string(b[:end])
			} else {
				outStr = hex.EncodeToString(b[:end])
			}
			if zeroCount > 0 {
				outStr += fmt.Sprintf("...(truncated, %d zero bytes omitted, total %d bytes)", zeroCount, totalLen)
			}
			result = outStr
		} else if v.Kind() == reflect.Slice && v.IsNil() {
			result = nil
		} else {
			maxElem := d.cs.MaxElementsPerContainer
			if maxElem <= 0 {
				maxElem = 1000 // 默认值
			}
			arr := make([]interface{}, 0, v.Len())
			limit := v.Len()
			if limit > maxElem {
				limit = maxElem
			}
			for i := 0; i < limit; i++ {
				elem := v.Index(i)
				if d.cs.SkipNilValues && (elem.Kind() == reflect.Ptr || elem.Kind() == reflect.Interface || elem.Kind() == reflect.Map || elem.Kind() == reflect.Slice) && elem.IsNil() {
					continue
				}
				arr = append(arr, d.dumpWithDepth(elem, depth+1))
			}
			if v.Len() > maxElem {
				arr = append(arr, map[string]interface{}{
					"__truncated__": true,
					"omitted":       v.Len() - maxElem,
					"len":           v.Len(),
					"cap":           v.Cap(),
					"type":          v.Type().String(),
				})
			}
			result = arr
		}
	case reflect.Chan:
		if v.IsNil() {
			result = nil
		} else {
			// 输出类型和地址占位
			result = fmt.Sprintf("<chan %s %#x>", v.Type().String(), v.Pointer())
		}
	case reflect.Func:
		if v.IsNil() {
			result = nil
		} else {
			// 输出类型和地址占位
			result = fmt.Sprintf("<func %s %#x>", v.Type().String(), v.Pointer())
		}
	case reflect.UnsafePointer:
		if v.IsNil() {
			result = nil
		} else {
			// 输出类型和地址占位
			result = fmt.Sprintf("<unsafe.Pointer %#x>", v.Pointer())
		}
	default:
		// 其它不支持的类型，输出类型信息
		result = fmt.Sprintf("<unsupported %s>", v.Type().String())
	}
	return result
}

// 判断是否纯ASCII或常用控制字符（\n, \r, \t）
func isPrintableOrControlASCII(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		if (c < 32 && c != 10 && c != 13 && c != 9) || c > 126 {
			return false
		}
	}
	return true
}

// 判断是否为JSON
func isJSON(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	if b[0] == '{' || b[0] == '[' {
		return true
	}
	return false
}

// 判断是否为UTF-8
func isUTF8(b []byte) bool {
	return utf8.Valid(b)
}

// 工具函数：判断类型是否为深层结构
func isDeepType(k reflect.Kind) bool {
	switch k {
	case reflect.Ptr, reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		return true
	default:
		return false
	}
}

// 工具函数：生成截断信息（支持len/cap）
func truncatedContainerInfo(typ string, depth, maxDepth, l, c int) map[string]interface{} {
	info := map[string]interface{}{
		"__truncated__": true,
		"type":          typ,
		"depth":         depth,
		"max_depth":     maxDepth,
	}
	if l > 0 {
		info["len"] = l
	}
	if c > 0 {
		info["cap"] = c
	}
	return info
}

// 工具函数：结构体字段递归转储
func dumpStructFields(d *dumpState, v reflect.Value, depth int) map[string]interface{} {
	m := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)
		var fieldResult interface{}

		if !field.IsExported() {
			// 只要允许unsafe，直接用unsafe读取，并递归dump
			if d.cs != nil {
				fv := bypassUnsafeReflectValue(field, v)
				// 如果返回的是字符串，说明无法读取，否则递归dump
				if fv.Kind() == reflect.String && fv.Type() == reflect.TypeOf("") {
					fieldResult = fv.Interface()
				} else {
					fieldResult = d.dumpWithDepth(fv, depth+1)
				}
			} else {
				fieldResult = "<unexported>"
			}
		} else {
			if d.cs.SkipNilValues && (fieldValue.Kind() == reflect.Ptr || fieldValue.Kind() == reflect.Interface || fieldValue.Kind() == reflect.Map || fieldValue.Kind() == reflect.Slice) && fieldValue.IsNil() {
				continue
			}
			if d.cs.MaxDepth > 0 && isDeepType(fieldValue.Kind()) && depth+1 > d.cs.MaxDepth {
				l, c := 0, 0
				switch fieldValue.Kind() {
				case reflect.Slice, reflect.Array, reflect.Map:
					l = fieldValue.Len()
					if fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array {
						c = fieldValue.Cap()
					}
				}
				fieldResult = truncatedContainerInfo(fieldValue.Type().String(), depth+1, d.cs.MaxDepth, l, c)
			} else {
				fieldResult = d.dumpWithDepth(fieldValue, depth+1)
			}
			// 无论是否截断，若为slice/array/map类型，补充len/cap
			if fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array || fieldValue.Kind() == reflect.Map {
				if obj, ok := fieldResult.(map[string]interface{}); ok {
					obj["len"] = fieldValue.Len()
					if fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array {
						obj["cap"] = fieldValue.Cap()
					}
					fieldResult = obj
				}
			}
		}
		m[field.Name] = fieldResult
	}
	m["type"] = v.Type().String()
	return m
}

// 参考 go-spew 官方实现，利用 reflect.NewAt+unsafe.Pointer 读取未导出字段
// 只能在同包作用域下读取，跨包无效
func bypassUnsafeReflectValue(field reflect.StructField, v reflect.Value) reflect.Value {
	if !v.CanAddr() {
		return reflect.ValueOf("<unexported, not addressable>")
	}
	ptr := unsafe.Pointer(v.UnsafeAddr() + field.Offset)
	fv := reflect.NewAt(field.Type, ptr).Elem()
	return fv
}

// 工具函数：map字段递归转储
func dumpMapFields(d *dumpState, v reflect.Value, depth int) map[string]interface{} {
	m := make(map[string]interface{})
	for _, k := range v.MapKeys() {
		val := v.MapIndex(k)
		if d.cs.SkipNilValues && (val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface || val.Kind() == reflect.Map || val.Kind() == reflect.Slice) && val.IsNil() {
			continue
		}
		var valResult interface{}
		if d.cs.MaxDepth > 0 && isDeepType(val.Kind()) && depth+1 > d.cs.MaxDepth {
			l, c := 0, 0
			switch val.Kind() {
			case reflect.Slice, reflect.Array, reflect.Map:
				l = val.Len()
				if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
					c = val.Cap()
				}
			}
			valResult = truncatedContainerInfo(val.Type().String(), depth+1, d.cs.MaxDepth, l, c)
		} else {
			valResult = d.dumpWithDepth(val, depth+1)
		}
		// 无论是否截断，若为slice/array/map类型，补充len/cap
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array || val.Kind() == reflect.Map {
			if obj, ok := valResult.(map[string]interface{}); ok {
				obj["len"] = val.Len()
				if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
					obj["cap"] = val.Cap()
				}
				valResult = obj
			}
		}
		m[fmt.Sprint(k.Interface())] = valResult
	}
	return m
}

// fdump 是一个辅助函数，用于合并来自各种公共方法的逻辑，这些方法采用不同的写入器和配置状态。
// 现在只支持JSON输出模式
// 参数:
//   - cs: 配置状态
//   - w: 输出写入器
//   - a: 要转储的参数列表
//
// 功能: 对每个参数进行JSON转储，处理nil值的特殊情况
func fdump(cs *ConfigState, w io.Writer, a ...interface{}) {
	for _, arg := range a {
		d := getDumpState(cs)
		result := d.dump(reflect.ValueOf(arg))
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.Encode(map[string]interface{}{"value": result})
		putDumpState(d)
	}
}

// Fdump 将传入的参数格式化并显示到io.Writer w。格式与Dump完全相同。
// 参数:
//   - w: 输出写入器
//   - a: 要转储的参数列表
//
// 功能: 使用全局配置将参数转储到指定的写入器
func Fdump(w io.Writer, a ...interface{}) {
	fdump(&Config, w, a...)
}

// Sdump 返回一个字符串，其中传入的参数格式与Dump完全相同。
// 参数:
//   - a: 要转储的参数列表
//
// 返回值:
//   - string: 格式化后的字符串
//
// 功能: 将转储结果作为字符串返回，而不是输出到标准输出
func Sdump(a ...interface{}) string {
	var buf bytes.Buffer
	fdump(&Config, &buf, a...)
	return buf.String()
}

/*
Dump 将传入的参数显示到标准输出，带有换行符、可自定义的缩进和额外的调试信息，
如完整的类型和用于间接访问最终值的所有指针地址。它提供了以下优于fmt包内置打印功能的特性：

  - 指针被解引用和跟踪
  - 循环数据结构被检测并正确处理
  - 自定义Stringer/error接口可选地被调用，包括未导出的类型
  - 仅通过指针接收器实现Stringer/error接口的自定义类型在传递非指针变量时可选地被调用
  - 字节数组和切片像hexdump -C命令一样转储，包括偏移量、十六进制字节值和ASCII输出

配置选项由导出的包全局变量spew.Config控制。请参阅ConfigState的选项文档。

如果您希望转储到任意io.Writer，请参阅Fdump；如果您希望将格式化结果作为字符串获取，请参阅Sdump。

参数:
  - a: 要转储的参数列表

功能: 将参数转储到标准输出
*/
func Dump(a ...interface{}) {
	fdump(&Config, os.Stdout, a...)
}

// ToJSON 返回传入参数的JSON表示。
// 参数:
//   - a: 要转换为JSON的参数列表
//
// 返回值:
//   - string: JSON格式的字符串
//
// 功能: 将Go对象转换为JSON字符串表示
func ToJSON(a ...interface{}) string {
	cs := ConfigState{Indent: " "}
	var buf bytes.Buffer
	fdump(&cs, &buf, a...)
	return buf.String()
}

// SdumpJSON 返回传入参数的JSON表示。
// 这等同于ToJSON，但保持与其他Sdump函数的命名一致性。
// 参数:
//   - a: 要转换为JSON的参数列表
//
// 返回值:
//   - string: JSON格式的字符串
//
// 功能: ToJSON的别名函数，保持命名一致性
func SdumpJSON(a ...interface{}) string {
	return ToJSON(a...)
}

// ToJSON 是ConfigState的便利方法，返回传入参数的JSON表示。
// 参数:
//   - a: 要转换为JSON的参数列表
//
// 返回值:
//   - string: JSON格式的字符串
//
// 功能: 在特定配置状态下将Go对象转换为JSON字符串
func (c *ConfigState) ToJSON(a ...interface{}) string {
	var buf bytes.Buffer
	fdump(c, &buf, a...)
	return buf.String()
}

// GetPoolStats 返回内存池的统计信息（用于调试和性能监控）
// 注意：这是一个调试函数，在生产代码中应谨慎使用
func GetPoolStats() (inUse int, available int) {
	// 由于sync.Pool没有直接暴露统计信息的API，
	// 我们通过临时获取和归还对象来检查池的状态
	var borrowed []*dumpState

	// 尝试借用最多10个对象来估算可用对象数量
	for i := 0; i < 10; i++ {
		if d := dumpStatePool.Get().(*dumpState); d != nil {
			borrowed = append(borrowed, d)
		} else {
			break
		}
	}

	available = len(borrowed)

	// 归还所有借用的对象
	for _, d := range borrowed {
		dumpStatePool.Put(d)
	}

	return 0, available // inUse 无法准确获取，返回0
}

// DumpToJSON 递归转储对象为 JSON 字符串，兼容原有 spew 行为
func DumpToJSON(v interface{}, cs *ConfigState) (string, error) {
	d := &dumpState{
		cs:    cs,
		depth: 0,
	}
	root := d.dump(reflect.ValueOf(v))
	data, err := json.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
