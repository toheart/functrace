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
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"

	"github.com/tidwall/sjson"
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
)

// dumpState包含转储操作状态的信息。
// 用于在递归转储过程中维护状态信息，包括输出流、深度、指针引用、配置状态等。
type dumpState struct {
	w                io.Writer       // 输出流
	depth            int             // 当前递归深度
	pointers         map[uintptr]int // 已处理的指针映射，用于检测循环引用
	ignoreNextType   bool            // 是否忽略下一个类型输出
	ignoreNextIndent bool            // 是否忽略下一个缩进
	cs               *ConfigState    // 配置状态
	jsonOutput       string          // JSON输出字符串
	currentPath      string          // 当前JSON路径
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

// indent 根据深度级别和cs.Indent选项执行缩进。
// 如果ignoreNextIndent为true，则跳过此次缩进并重置该标志。
func (d *dumpState) indent() {
	if d.ignoreNextIndent {
		d.ignoreNextIndent = false
		return
	}
	d.w.Write(bytes.Repeat([]byte(d.cs.Indent), d.depth))
}

// setJSONValue 将值添加到JSON输出中
// 参数:
//   - path: JSON路径
//   - value: 要设置的值
//
// 功能: 使用sjson库将键值对添加到JSON输出字符串中，如果SkipNilValues为true且值为nil则跳过
func (d *dumpState) setJSONValue(path string, value interface{}) {
	// 如果SkipNilValues为true且值为nil，则跳过设置
	if d.cs.SkipNilValues && value == nil {
		return
	}

	var err error
	if d.jsonOutput == "" {
		d.jsonOutput = "{}"
	}

	d.jsonOutput, err = sjson.Set(d.jsonOutput, path, value)
	if err != nil {
		// 如果设置JSON失败，我们仍然尝试继续
		fmt.Fprintf(d.w, "/* JSON set error for path %s: %v */", path, err)
	}
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
	// 始终返回十六进制表示，不再尝试转换为字符串
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

// dumpPtr 通过必要的间接引用来处理指针的格式化。
// 参数:
//   - v: 指针类型的reflect.Value
//
// 功能: 处理指针链，检测循环引用，显示类型信息和指针地址，递归转储指向的值
func (d *dumpState) dumpPtr(v reflect.Value) {
	// Remove pointers at or below the current depth from map used to detect
	// circular refs.
	for k, depth := range d.pointers {
		if depth >= d.depth {
			delete(d.pointers, k)
		}
	}

	// Keep list of all dereferenced pointers to show later.
	pointerChain := make([]uintptr, 0)

	// Figure out how many levels of indirection there are by dereferencing
	// pointers and unpacking interfaces down the chain while detecting circular
	// references.
	nilFound := false
	cycleFound := false
	indirects := 0
	ve := v
	for ve.Kind() == reflect.Ptr {
		if ve.IsNil() {
			nilFound = true
			break
		}
		indirects++
		addr := ve.Pointer()
		pointerChain = append(pointerChain, addr)
		if pd, ok := d.pointers[addr]; ok && pd < d.depth {
			cycleFound = true
			indirects--
			break
		}
		d.pointers[addr] = d.depth

		ve = ve.Elem()
		if ve.Kind() == reflect.Interface {
			if ve.IsNil() {
				nilFound = true
				break
			}
			ve = ve.Elem()
		}
	}

	// Display type information.
	d.w.Write(openParenBytes)
	d.w.Write(bytes.Repeat(asteriskBytes, indirects))
	d.w.Write([]byte(ve.Type().String()))
	d.w.Write(closeParenBytes)

	// Display pointer information.
	if !d.cs.DisablePointerAddresses && len(pointerChain) > 0 {
		d.w.Write(openParenBytes)
		for i, addr := range pointerChain {
			if i > 0 {
				d.w.Write(pointerChainBytes)
			}
			printHexPtr(d.w, addr)
		}
		d.w.Write(closeParenBytes)
	}

	// Display dereferenced value.
	d.w.Write(openParenBytes)
	switch {
	case nilFound:
		d.w.Write(nilAngleBytes)

	case cycleFound:
		d.w.Write(circularBytes)

	default:
		d.ignoreNextType = true
		d.dump(ve)
	}
	d.w.Write(closeParenBytes)
}

// dumpSlice 处理数组和切片的格式化。字节（反射下的uint8）数组和切片以hexdump -C方式转储。
// 参数:
//   - v: 数组或切片类型的reflect.Value
//
// 功能: 处理数组和切片的转储，支持字节数组的十六进制显示，限制显示元素数量，支持JSON输出
func (d *dumpState) dumpSlice(v reflect.Value) {
	// Determine whether this type should be hex dumped or not.  Also,
	// for types which should be hexdumped, try to use the underlying data
	// first, then fall back to trying to convert them to a uint8 slice.
	var buf []uint8
	doConvert := false
	numEntries := v.Len()

	// 限制最多显示10个元素
	const maxElements = 10
	var truncated bool
	if numEntries > maxElements {
		truncated = true
		numEntries = maxElements
	}

	if numEntries > 0 {
		vt := v.Index(0).Type()
		vts := vt.String()
		switch {
		// C types that need to be converted.
		case cCharRE.MatchString(vts):
			fallthrough
		case cUnsignedCharRE.MatchString(vts):
			fallthrough
		case cUint8tCharRE.MatchString(vts):
			doConvert = true

		// Try to use existing uint8 slices and fall back to converting
		// and copying if that fails.
		case vt.Kind() == reflect.Uint8:
			// 我们需要一个可以寻址的接口来将类型转换为字节切片。然而，reflect包在某些情况下，如
			// 未导出结构体字段，不会给我们一个接口，以执行可见性规则。我们使用不安全的方法，仅在可用时，
			// 来绕过这些限制，因为这个包不修改值。
			vs := v
			if !vs.CanInterface() || !vs.CanAddr() {
				vs = unsafeReflectValue(vs)
			}
			if !UnsafeDisabled {
				vs = vs.Slice(0, numEntries)

				// Use the existing uint8 slice if it can be
				// type asserted.
				iface := vs.Interface()
				if slice, ok := iface.([]uint8); ok {
					buf = slice
					break
				}
			}

			// The underlying data needs to be converted if it can't
			// be type asserted to a uint8 slice.
			doConvert = true
		}

		// Copy and convert the underlying type if needed.
		if doConvert && vt.ConvertibleTo(uint8Type) {
			// Convert and copy each element into a uint8 byte
			// slice.
			buf = make([]uint8, numEntries)
			for i := 0; i < numEntries; i++ {
				vv := v.Index(i)
				converted := vv.Convert(uint8Type)
				if converted.CanUint() {
					buf[i] = uint8(converted.Uint())
				} else {
					buf[i] = 0
				}
			}
		}
	}

	// Recursively call dump for each item.
	if d.cs.EnableJSONOutput {
		d.setJSONValue(d.currentPath, getUint8String(buf))
		// 为JSON输出准备数组
		for i := 0; i < numEntries; i++ {
			// 获取元素值
			elemValue := d.unpackValue(v.Index(i))

			// 如果值为nil且SkipNilValues为true，则跳过该元素
			if d.cs.SkipNilValues &&
				(elemValue.Kind() == reflect.Interface || elemValue.Kind() == reflect.Ptr ||
					elemValue.Kind() == reflect.Map || elemValue.Kind() == reflect.Slice ||
					elemValue.Kind() == reflect.Chan) && elemValue.IsNil() {
				continue
			}

			// 构建数组元素的路径
			indexPath := fmt.Sprintf("%s[%d]", d.currentPath, i)
			oldPath := d.currentPath
			d.currentPath = indexPath

			// 递归调用dump
			d.dump(elemValue)

			// 恢复路径
			d.currentPath = oldPath

			if i < (numEntries - 1) {
				d.w.Write(commaNewlineBytes)
			} else {
				d.w.Write(newlineBytes)
			}
		}

		// 如果有元素被截断，添加提示信息
		if truncated {
			truncatedPath := fmt.Sprintf("%s.truncated", d.currentPath)
			totalLen := v.Len()
			d.setJSONValue(truncatedPath, fmt.Sprintf("... 还有 %d 个元素被截断", totalLen-maxElements))
		}
	} else {
		// 原始的非JSON输出逻辑
		for i := 0; i < numEntries; i++ {
			d.dump(d.unpackValue(v.Index(i)))
			if i < (numEntries - 1) {
				d.w.Write(commaNewlineBytes)
			} else {
				d.w.Write(newlineBytes)
			}
		}

		// 如果有元素被截断，添加提示信息
		if truncated {
			d.indent()
			totalLen := v.Len()
			d.w.Write([]byte(fmt.Sprintf("... 还有 %d 个元素被截断\n", totalLen-maxElements)))
		}
	}
}

// dump 是转储值的主要工作函数。它使用传入的反射值来确定我们正在处理什么类型的对象，并适当地格式化它。
// 这是一个递归函数，但是会检测并正确处理循环数据结构。
// 参数:
//   - v: 要转储的reflect.Value
//
// 功能: 根据值的类型进行相应的格式化输出，支持各种Go类型，包括基本类型、复合类型、指针等
func (d *dumpState) dump(v reflect.Value) {
	// Handle invalid reflect values immediately.
	kind := v.Kind()
	if kind == reflect.Invalid {
		d.w.Write(invalidAngleBytes)
		if d.cs.EnableJSONOutput {
			d.setJSONValue(d.currentPath, "invalid")
		}
		return
	}

	// Handle pointers specially.
	if kind == reflect.Ptr {
		d.indent()
		d.dumpPtr(v)
		return
	}

	// Print type information unless already handled elsewhere.
	if !d.ignoreNextType {
		d.indent()
		d.w.Write(openParenBytes)
		typeStr := v.Type().String()
		d.w.Write([]byte(typeStr))
		d.w.Write(closeParenBytes)
		d.w.Write(spaceBytes)

		// 如果启用了JSON输出，添加类型信息到JSON
		if d.cs.EnableJSONOutput {
			typePath := d.currentPath
			if typePath != "" {
				typePath = typePath + ".type"
			} else {
				typePath = "type"
			}
			d.setJSONValue(typePath, typeStr)
		}
	}
	d.ignoreNextType = false

	// Display length and capacity if the built-in len and cap functions
	// work with the value's kind and the len/cap itself is non-zero.
	valueLen, valueCap := 0, 0
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Chan:
		valueLen, valueCap = v.Len(), v.Cap()
	case reflect.Map, reflect.String:
		valueLen = v.Len()
	}
	if valueLen != 0 || !d.cs.DisableCapacities && valueCap != 0 {
		d.w.Write(openParenBytes)
		if valueLen != 0 {
			d.w.Write(lenEqualsBytes)
			printInt(d.w, int64(valueLen), 10)

			// 如果启用了JSON输出，添加长度信息到JSON
			if d.cs.EnableJSONOutput {
				lenPath := d.currentPath
				if lenPath != "" {
					lenPath = lenPath + ".length"
				} else {
					lenPath = "length"
				}
				d.setJSONValue(lenPath, valueLen)
			}
		}
		if !d.cs.DisableCapacities && valueCap != 0 {
			if valueLen != 0 {
				d.w.Write(spaceBytes)
			}
			d.w.Write(capEqualsBytes)
			printInt(d.w, int64(valueCap), 10)

			// 如果启用了JSON输出，添加容量信息到JSON
			if d.cs.EnableJSONOutput {
				capPath := d.currentPath
				if capPath != "" {
					capPath = capPath + ".capacity"
				} else {
					capPath = "capacity"
				}
				d.setJSONValue(capPath, valueCap)
			}
		}
		d.w.Write(closeParenBytes)
		d.w.Write(spaceBytes)
	}

	// Call Stringer/error interfaces if they exist and the handle methods flag
	// is enabled
	if !d.cs.DisableMethods {
		if (kind != reflect.Invalid) && (kind != reflect.Interface) {
			if handled := handleMethods(d.cs, d.w, v); handled {
				if d.cs.EnableJSONOutput {
					// 对于实现了Stringer/error接口的对象，可以尝试获取字符串表示
					if v.CanInterface() {
						val := v.Interface()
						var str string
						if stringer, ok := val.(fmt.Stringer); ok {
							str = stringer.String()
						} else if err, ok := val.(error); ok {
							str = err.Error()
						}
						if str != "" {
							d.setJSONValue(d.currentPath, str)
						}
					}
				}
				return
			}
		}
	}

	switch kind {
	case reflect.Invalid:
		// Do nothing.  We should never get here since invalid has already
		// been handled above.

	case reflect.Bool:
		val := v.Bool()
		printBool(d.w, val)
		if d.cs.EnableJSONOutput {
			d.setJSONValue(d.currentPath, val)
		}

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		if v.CanInt() {
			val := v.Int()
			printInt(d.w, val, 10)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, val)
			}
		} else {
			d.w.Write([]byte("<invalid int value>"))
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid int value")
			}
		}

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		if v.IsValid() && v.CanUint() {
			val := v.Uint()
			printUint(d.w, val, 10)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, val)
			}
		} else {
			d.w.Write([]byte("<invalid uint value>"))
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid uint value")
			}
		}

	case reflect.Float32:
		if v.CanFloat() {
			val := v.Float()
			printFloat(d.w, val, 32)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, val)
			}
		} else {
			d.w.Write([]byte("<invalid float32 value>"))
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid float32 value")
			}
		}

	case reflect.Float64:
		if v.CanFloat() {
			val := v.Float()
			printFloat(d.w, val, 64)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, val)
			}
		} else {
			d.w.Write([]byte("<invalid float64 value>"))
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid float64 value")
			}
		}

	case reflect.Complex64:
		if v.CanComplex() {
			val := v.Complex()
			printComplex(d.w, val, 32)
			if d.cs.EnableJSONOutput {
				// JSON不支持复数，转换为字符串
				d.setJSONValue(d.currentPath, fmt.Sprintf("%v", val))
			}
		} else {
			d.w.Write([]byte("<invalid complex64 value>"))
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid complex64 value")
			}
		}

	case reflect.Complex128:
		if v.CanComplex() {
			val := v.Complex()
			printComplex(d.w, val, 64)
			if d.cs.EnableJSONOutput {
				// JSON不支持复数，转换为字符串
				d.setJSONValue(d.currentPath, fmt.Sprintf("%v", val))
			}
		} else {
			d.w.Write([]byte("<invalid complex128 value>"))
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid complex128 value")
			}
		}

	case reflect.Slice:
		if v.IsNil() {
			d.w.Write(nilAngleBytes)
			if d.cs.EnableJSONOutput && !d.cs.SkipNilValues {
				d.setJSONValue(d.currentPath, nil)
			}
			break
		}
		fallthrough

	case reflect.Array:
		d.w.Write(openBraceNewlineBytes)
		d.depth++
		if (d.cs.MaxDepth != 0) && (d.depth > d.cs.MaxDepth) {
			d.indent()
			d.w.Write(maxNewlineBytes)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "max depth reached")
			}
		} else {
			oldPath := d.currentPath
			d.dumpSlice(v)
			d.currentPath = oldPath
		}
		d.depth--
		d.indent()
		d.w.Write(closeBraceBytes)

	case reflect.String:
		val := v.String()
		d.w.Write([]byte(strconv.Quote(val)))
		if d.cs.EnableJSONOutput {
			d.setJSONValue(d.currentPath, val)
		}

	case reflect.Interface:
		// The only time we should get here is for nil interfaces due to
		// unpackValue calls.
		if v.IsNil() {
			d.w.Write(nilAngleBytes)
			if d.cs.EnableJSONOutput && !d.cs.SkipNilValues {
				d.setJSONValue(d.currentPath, nil)
			}
		}

	case reflect.Ptr:
		// Do nothing.  We should never get here since pointers have already
		// been handled above.

	case reflect.Map:
		// nil maps should be indicated as different than empty maps
		if v.IsNil() {
			d.w.Write(nilAngleBytes)
			if d.cs.EnableJSONOutput && !d.cs.SkipNilValues {
				d.setJSONValue(d.currentPath, nil)
			}
			break
		}

		d.w.Write(openBraceNewlineBytes)
		d.depth++
		if (d.cs.MaxDepth != 0) && (d.depth > d.cs.MaxDepth) {
			d.indent()
			d.w.Write(maxNewlineBytes)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "max depth reached")
			}
		} else {
			numEntries := v.Len()
			keys := v.MapKeys()
			if d.cs.SortKeys {
				sortValues(keys, d.cs)
			}

			// 限制最多显示10个键值对
			const maxMapElements = 10
			var mapTruncated bool
			if len(keys) > maxMapElements {
				mapTruncated = true
				keys = keys[:maxMapElements]
				numEntries = maxMapElements
			}

			for i, key := range keys {
				d.dump(d.unpackValue(key))
				d.w.Write(colonSpaceBytes)
				d.ignoreNextIndent = true

				// 为JSON准备key
				var keyStr string
				k := d.unpackValue(key)
				switch k.Kind() {
				case reflect.String:
					keyStr = k.String()
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					keyStr = fmt.Sprintf("%d", k.Int())
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					if k.CanUint() {
						keyStr = fmt.Sprintf("%d", k.Uint())
					} else {
						keyStr = "invalid_uint"
					}
				default:
					// 对于其他类型的键，使用其字符串表示
					if k.CanInterface() {
						keyStr = fmt.Sprintf("%v", k.Interface())
					} else {
						keyStr = fmt.Sprintf("%v", k.String())
					}
				}

				// 保存当前路径并更新
				oldPath := d.currentPath
				if d.currentPath != "" {
					d.currentPath = d.currentPath + "." + keyStr
				} else {
					d.currentPath = keyStr
				}

				// 获取值
				mapValue := d.unpackValue(v.MapIndex(key))

				// 如果值为nil且SkipNilValues为true，则跳过该键值对
				if d.cs.SkipNilValues && d.cs.EnableJSONOutput &&
					(mapValue.Kind() == reflect.Interface || mapValue.Kind() == reflect.Ptr ||
						mapValue.Kind() == reflect.Map || mapValue.Kind() == reflect.Slice ||
						mapValue.Kind() == reflect.Chan) && mapValue.IsNil() {
					// 恢复路径
					d.currentPath = oldPath
					continue
				}
				// 转储值
				d.dump(mapValue)

				// 恢复路径
				d.currentPath = oldPath

				if i < (numEntries - 1) {
					d.w.Write(commaNewlineBytes)
				} else {
					d.w.Write(newlineBytes)
				}
			}

			// 如果有键值对被截断，添加提示信息
			if mapTruncated {
				if d.cs.EnableJSONOutput {
					truncatedPath := fmt.Sprintf("%s.truncated", d.currentPath)
					totalLen := v.Len()
					d.setJSONValue(truncatedPath, fmt.Sprintf("... 还有 %d 个键值对被截断", totalLen-maxMapElements))
				} else {
					d.indent()
					totalLen := v.Len()
					d.w.Write([]byte(fmt.Sprintf("... 还有 %d 个键值对被截断\n", totalLen-maxMapElements)))
				}
			}
		}
		d.depth--
		d.indent()
		d.w.Write(closeBraceBytes)

	case reflect.Struct:
		d.w.Write(openBraceNewlineBytes)
		d.depth++
		if (d.cs.MaxDepth != 0) && (d.depth > d.cs.MaxDepth) {
			d.indent()
			d.w.Write(maxNewlineBytes)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "max depth reached")
			}
		} else {
			vt := v.Type()
			numFields := v.NumField()
			for i := 0; i < numFields; i++ {
				d.indent()
				vtf := vt.Field(i)
				fieldName := vtf.Name
				d.w.Write([]byte(fieldName))
				d.w.Write(colonSpaceBytes)
				d.ignoreNextIndent = true

				// 保存当前路径并更新
				oldPath := d.currentPath
				if d.currentPath != "" {
					d.currentPath = d.currentPath + "." + fieldName
				} else {
					d.currentPath = fieldName
				}

				// 获取字段值
				fieldValue := d.unpackValue(v.Field(i))

				// 如果字段值为nil且SkipNilValues为true，则跳过该字段
				if d.cs.SkipNilValues && d.cs.EnableJSONOutput &&
					(fieldValue.Kind() == reflect.Interface || fieldValue.Kind() == reflect.Ptr ||
						fieldValue.Kind() == reflect.Map || fieldValue.Kind() == reflect.Slice ||
						fieldValue.Kind() == reflect.Chan) && fieldValue.IsNil() {
					// 恢复路径
					d.currentPath = oldPath
					continue
				}

				// 转储字段值
				d.dump(fieldValue)

				// 恢复路径
				d.currentPath = oldPath

				if i < (numFields - 1) {
					d.w.Write(commaNewlineBytes)
				} else {
					d.w.Write(newlineBytes)
				}
			}
		}
		d.depth--
		d.indent()
		d.w.Write(closeBraceBytes)

	case reflect.Uintptr:
		if v.IsValid() && v.CanUint() {
			val := uintptr(v.Uint())
			printHexPtr(d.w, val)
			if d.cs.EnableJSONOutput {
				// 将指针转换为字符串表示
				d.setJSONValue(d.currentPath, fmt.Sprintf("0x%x", val))
			}
		} else {
			d.w.Write(invalidAngleBytes)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid uintptr")
			}
		}

	case reflect.UnsafePointer, reflect.Chan, reflect.Func:
		if v.IsValid() {
			val := v.Pointer()
			printHexPtr(d.w, val)
			if d.cs.EnableJSONOutput {
				// 将指针转换为字符串表示
				d.setJSONValue(d.currentPath, fmt.Sprintf("0x%x", val))
			}
		} else {
			d.w.Write(invalidAngleBytes)
			if d.cs.EnableJSONOutput {
				d.setJSONValue(d.currentPath, "invalid pointer")
			}
		}

	// There were not any other types at the time this code was written, but
	// fall back to letting the default fmt package handle it in case any new
	// types are added.
	default:
		var val interface{}
		if v.CanInterface() {
			// 使用安全调用来防止panic
			if result, success := d.safeReflectCall(func() interface{} {
				return v.Interface()
			}, "invalid interface value"); success {
				val = result
				fmt.Fprintf(d.w, "%v", val)
			} else {
				val = result
				d.w.Write([]byte(fmt.Sprintf("<%s>", result)))
			}
		} else {
			// 即使是String()也可能panic，所以也要保护
			if result, success := d.safeReflectCall(func() interface{} {
				return v.String()
			}, "invalid string value"); success {
				val = result
				fmt.Fprintf(d.w, "%v", val)
			} else {
				val = result
				d.w.Write([]byte(fmt.Sprintf("<%s>", result)))
			}
		}
		if d.cs.EnableJSONOutput {
			d.setJSONValue(d.currentPath, val)
		}
	}
}

// fdump 是一个辅助函数，用于合并来自各种公共方法的逻辑，这些方法采用不同的写入器和配置状态。
// 参数:
//   - cs: 配置状态
//   - w: 输出写入器
//   - a: 要转储的参数列表
//
// 功能: 对每个参数进行转储，支持JSON和常规输出模式，处理nil值的特殊情况
func fdump(cs *ConfigState, w io.Writer, a ...interface{}) {
	for _, arg := range a {
		if arg == nil {
			// 如果SkipNilValues为true，则跳过nil值
			if cs.SkipNilValues {
				continue
			}

			if cs.EnableJSONOutput {
				json := "{\"value\": null}"
				w.Write([]byte(json))
				w.Write(newlineBytes)
			} else {
				w.Write(interfaceBytes)
				w.Write(spaceBytes)
				w.Write(nilAngleBytes)
				w.Write(newlineBytes)
			}
			continue
		}

		d := dumpState{w: w, cs: cs}
		d.pointers = make(map[uintptr]int)

		// 如果启用了JSON输出，初始化JSON字符串
		if cs.EnableJSONOutput {
			d.jsonOutput = "{}"
			d.currentPath = "value"

			// 在JSON模式下，我们不直接写入输出
			// 创建一个空缓冲区，后面会忽略它的内容
			d.w = &bytes.Buffer{}
		}
		v := reflect.ValueOf(arg)
		d.dump(v)

		// 如果启用了JSON输出，只写入JSON字符串
		if cs.EnableJSONOutput {
			w.Write([]byte(d.jsonOutput))
			w.Write(newlineBytes)
		} else {
			d.w.Write(newlineBytes)
		}
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
// 它临时启用EnableJSONOutput选项来生成JSON输出。
// 参数:
//   - a: 要转换为JSON的参数列表
//
// 返回值:
//   - string: JSON格式的字符串
//
// 功能: 将Go对象转换为JSON字符串表示
func ToJSON(a ...interface{}) string {
	cs := *NewDefaultConfig()
	cs.EnableJSONOutput = true
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

// ToJSON 是ConfigState的便利方法，它启用JSON输出并返回传入参数的JSON表示。
// 参数:
//   - a: 要转换为JSON的参数列表
//
// 返回值:
//   - string: JSON格式的字符串
//
// 功能: 在特定配置状态下将Go对象转换为JSON字符串，转换后恢复原始的JSON输出设置
func (c *ConfigState) ToJSON(a ...interface{}) string {
	originalJSONSetting := c.EnableJSONOutput
	c.EnableJSONOutput = true
	var buf bytes.Buffer
	fdump(c, &buf, a...)
	c.EnableJSONOutput = originalJSONSetting
	return buf.String()
}
