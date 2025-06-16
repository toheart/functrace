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
	// uint8Type is a reflect.Type representing a uint8.  It is used to
	// convert cgo types to uint8 slices for hexdumping.
	uint8Type = reflect.TypeOf(uint8(0))

	// cCharRE is a regular expression that matches a cgo char.
	// It is used to detect character arrays to hexdump them.
	cCharRE = regexp.MustCompile(`^.*\._Ctype_char$`)

	// cUnsignedCharRE is a regular expression that matches a cgo unsigned
	// char.  It is used to detect unsigned character arrays to hexdump
	// them.
	cUnsignedCharRE = regexp.MustCompile(`^.*\._Ctype_unsignedchar$`)

	// cUint8tCharRE is a regular expression that matches a cgo uint8_t.
	// It is used to detect uint8_t arrays to hexdump them.
	cUint8tCharRE = regexp.MustCompile(`^.*\._Ctype_uint8_t$`)
)

// dumpState contains information about the state of a dump operation.
type dumpState struct {
	w                io.Writer
	depth            int
	pointers         map[uintptr]int
	ignoreNextType   bool
	ignoreNextIndent bool
	cs               *ConfigState
	jsonOutput       string
	currentPath      string
}

// safeReflectCall 安全地执行反射操作，捕获可能的panic
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

// indent performs indentation according to the depth level and cs.Indent
// option.
func (d *dumpState) indent() {
	if d.ignoreNextIndent {
		d.ignoreNextIndent = false
		return
	}
	d.w.Write(bytes.Repeat([]byte(d.cs.Indent), d.depth))
}

// setJSONValue 将值添加到JSON输出中
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

// 获取uint8数组或切片的字符串表示
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

// unpackValue returns values inside of non-nil interfaces when possible.
// This is useful for data types like structs, arrays, slices, and maps which
// can contain varying types packed inside an interface.
func (d *dumpState) unpackValue(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	return v
}

// dumpPtr handles formatting of pointers by indirecting them as necessary.
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

// dumpSlice handles formatting of arrays and slices.  Byte (uint8 under
// reflection) arrays and slices are dumped in hexdump -C fashion.
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

// dump is the main workhorse for dumping a value.  It uses the passed reflect
// value to figure out what kind of object we are dealing with and formats it
// appropriately.  It is a recursive function, however circular data structures
// are detected and handled properly.
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

// fdump is a helper function to consolidate the logic from the various public
// methods which take varying writers and config states.
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

// Fdump formats and displays the passed arguments to io.Writer w.  It formats
// exactly the same as Dump.
func Fdump(w io.Writer, a ...interface{}) {
	fdump(&Config, w, a...)
}

// Sdump returns a string with the passed arguments formatted exactly the same
// as Dump.
func Sdump(a ...interface{}) string {
	var buf bytes.Buffer
	fdump(&Config, &buf, a...)
	return buf.String()
}

/*
Dump displays the passed parameters to standard out with newlines, customizable
indentation, and additional debug information such as complete types and all
pointer addresses used to indirect to the final value.  It provides the
following features over the built-in printing facilities provided by the fmt
package:

  - Pointers are dereferenced and followed
  - Circular data structures are detected and handled properly
  - Custom Stringer/error interfaces are optionally invoked, including
    on unexported types
  - Custom types which only implement the Stringer/error interfaces via
    a pointer receiver are optionally invoked when passing non-pointer
    variables
  - Byte arrays and slices are dumped like the hexdump -C command which
    includes offsets, byte values in hex, and ASCII output

The configuration options are controlled by an exported package global,
spew.Config.  See ConfigState for options documentation.

See Fdump if you would prefer dumping to an arbitrary io.Writer or Sdump to
get the formatted result as a string.
*/
func Dump(a ...interface{}) {
	fdump(&Config, os.Stdout, a...)
}

// ToJSON returns a JSON representation of the passed arguments.
// It enables the EnableJSONOutput option temporarily to generate the JSON output.
func ToJSON(a ...interface{}) string {
	cs := *NewDefaultConfig()
	cs.EnableJSONOutput = true
	var buf bytes.Buffer
	fdump(&cs, &buf, a...)
	return buf.String()
}

// SdumpJSON returns a JSON representation of the passed arguments.
// This is equivalent to ToJSON but maintains naming consistency with other
// Sdump functions.
func SdumpJSON(a ...interface{}) string {
	return ToJSON(a...)
}

// ConfigState.ToJSON is a convenience method that enables JSON output and
// returns a JSON representation of the passed arguments.
func (c *ConfigState) ToJSON(a ...interface{}) string {
	originalJSONSetting := c.EnableJSONOutput
	c.EnableJSONOutput = true
	var buf bytes.Buffer
	fdump(c, &buf, a...)
	c.EnableJSONOutput = originalJSONSetting
	return buf.String()
}
