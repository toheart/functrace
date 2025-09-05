package objectdump

import (
	"encoding/hex"
	"fmt"
	"reflect"
)

// --- Complex Type Dumpers ---

type ptrDumper struct{}

func (d *ptrDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	// 基础有效性检查
	if !v.IsValid() {
		return fmt.Sprintf("<%s:invalid>", v.Type().String())
	}

	if v.Kind() != reflect.Ptr {
		return fmt.Sprintf("<%s:not_pointer>", v.Type().String())
	}

	// Circular reference detection
	if v.CanAddr() {
		addr := v.UnsafeAddr()
		if addr != 0 {
			if ds.visited == nil {
				ds.visited = make(map[uintptr]int)
			}
			if _, ok := ds.visited[addr]; ok {
				return fmt.Sprintf("<circular %s %#x>", v.Type().String(), addr)
			}
			ds.visited[addr] = depth
			defer delete(ds.visited, addr)
		}
	}

	if v.IsNil() {
		return nil
	}

	// 额外的指针安全性检查
	ptrAddr := v.Pointer()
	if ptrAddr == 0 {
		return fmt.Sprintf("<%s:null_pointer>", v.Type().String())
	}

	// 安全检查：验证指针是否可以安全解引用
	var result interface{}
	var hasError bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				// 如果解引用时 panic，返回安全描述
				result = fmt.Sprintf("<%s:unsafe_deref:%v>", v.Type().String(), r)
				hasError = true
			}
		}()

		// 先检查解引用后的值是否有效
		elem := v.Elem()
		if !elem.IsValid() {
			result = fmt.Sprintf("<%s:invalid_target>", v.Type().String())
			hasError = true
			return
		}

		if !hasError {
			result = ds.dumpWithDepth(elem, depth)
		}
	}()

	if hasError {
		return result
	}

	// 解引用指针不计入逻辑深度，避免多层 *T 过早触发 MaxDepth
	return result
}

type structDumper struct{}

func (d *structDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	m := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)
		var fieldResult interface{}

		if !field.IsExported() {
			if ds.cs.AllowUnexported {
				fv := bypassUnsafeReflectValue(field, v)
				if fv.Kind() == reflect.String && fv.Type() == reflect.TypeOf("") {
					fieldResult = fv.Interface()
				} else {
					fieldResult = ds.dumpWithDepth(fv, depth+1)
				}
			} else {
				fieldResult = "<unexported>"
			}
		} else {
			if ds.cs.SkipNilValues && (fieldValue.Kind() == reflect.Ptr || fieldValue.Kind() == reflect.Interface || fieldValue.Kind() == reflect.Map || fieldValue.Kind() == reflect.Slice) && fieldValue.IsNil() {
				continue
			}
			fieldResult = ds.dumpWithDepth(fieldValue, depth+1)
		}
		m[field.Name] = fieldResult
	}
	m["type"] = v.Type().String()
	return m
}

type mapDumper struct{}

func (d *mapDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	if v.IsNil() {
		return nil
	}

	// 当未开启 map 深度解析时，返回摘要信息（类型 + 长度 + 容量）
	if !ds.cs.ParseMapValues {
		return map[string]interface{}{
			"type": v.Type().String(),
			"len":  v.Len(),
		}
	}

	// 快速限制拷贝策略：只拷贝有限数量的键值对，减少并发访问时间
	maxElem := ds.cs.MaxElementsPerContainer
	if maxElem <= 0 {
		maxElem = 20 // 保守的默认限制，减少并发冲突
	}

	// 使用最快的方式：预分配结果map，快进快出
	result := make(map[string]interface{}, maxElem)
	var mapSize int
	var truncated bool

	// 原子操作：快速获取keys并立即拷贝前N个
	func() {
		defer func() {
			if r := recover(); r != nil {
				// 并发访问失败，返回错误信息
				result = map[string]interface{}{
					"error": "concurrent_map_access",
					"type":  v.Type().String(),
					"note":  "快速拷贝失败，map正在被修改",
				}
			}
		}()

		// 第一步：快速获取大小
		mapSize = v.Len()
		if mapSize == 0 {
			return // 空map，直接返回空result
		}

		// 第二步：快速获取keys（这是最危险的操作）
		allKeys := v.MapKeys()

		// 第三步：只处理前maxElem个元素，快进快出
		processCount := len(allKeys)
		if processCount > maxElem {
			processCount = maxElem
			truncated = true
		}

		// 第四步：批量快速拷贝，最小化访问时间
		for i := 0; i < processCount; i++ {
			key := allKeys[i]
			val := v.MapIndex(key)

			// 快速验证：如果值无效（被删除），跳过
			if !val.IsValid() {
				continue
			}

			// 快速跳过nil值（如果配置要求）
			if ds.cs.SkipNilValues && isNilValue(val) {
				continue
			}

			// 快速转换key并存储
			keyStr := fastKeyToString(key)
			result[keyStr] = ds.dumpWithDepth(val, depth+1)
		}
	}()

	// 添加截断信息
	if truncated && mapSize > maxElem {
		result["...truncated"] = fmt.Sprintf("(%d more items, total %d items)", mapSize-len(result), mapSize)
	}

	return result
}

// fastKeyToString 快速将key转换为字符串
func fastKeyToString(key reflect.Value) string {
	switch key.Kind() {
	case reflect.String:
		return key.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", key.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", key.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", key.Float())
	case reflect.Bool:
		if key.Bool() {
			return "true"
		}
		return "false"
	default:
		// 对于复杂类型，使用fmt.Sprint作为后备
		return fmt.Sprint(key.Interface())
	}
}

// isNilValue 快速检查值是否为nil
func isNilValue(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return val.IsNil()
	default:
		return false
	}
}

type sliceDumper struct{}

func (d *sliceDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	if v.Type().Elem().Kind() == reflect.Uint8 {
		return d.dumpByteSlice(ds, v)
	}

	if v.IsNil() {
		return nil
	}

	return d.dumpGenericSlice(ds, v, depth)
}

func (d *sliceDumper) dumpByteSlice(ds *dumpState, v reflect.Value) interface{} {
	return safeDumpBytes(v, "slice")
}

func (d *sliceDumper) dumpGenericSlice(ds *dumpState, v reflect.Value, depth int) interface{} {
	maxElem := ds.cs.MaxElementsPerContainer
	if maxElem <= 0 {
		maxElem = 1000 // Default limit
	}
	arr := make([]interface{}, 0, v.Len())
	limit := v.Len()
	if limit > maxElem {
		limit = maxElem
	}
	for i := 0; i < limit; i++ {
		elem := v.Index(i)
		if ds.cs.SkipNilValues && (elem.Kind() == reflect.Ptr || elem.Kind() == reflect.Interface || elem.Kind() == reflect.Map || elem.Kind() == reflect.Slice) && elem.IsNil() {
			continue
		}
		arr = append(arr, ds.dumpWithDepth(elem, depth+1))
	}
	if v.Len() > maxElem {
		arr = append(arr, fmt.Sprintf("...(truncated, %d items omitted, total %d, cap %d, type %s)", v.Len()-maxElem, v.Len(), v.Cap(), v.Type().String()))
	}
	return arr
}

type arrayDumper struct{}

func (d *arrayDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	if v.Type().Elem().Kind() == reflect.Uint8 {
		return d.dumpByteArray(ds, v)
	}

	return d.dumpGenericArray(ds, v, depth)
}

func (d *arrayDumper) dumpByteArray(ds *dumpState, v reflect.Value) interface{} {
	return safeDumpBytes(v, "array")
}

func (d *arrayDumper) dumpGenericArray(ds *dumpState, v reflect.Value, depth int) interface{} {
	maxElem := ds.cs.MaxElementsPerContainer
	if maxElem <= 0 {
		maxElem = 10 // Default limit
	}
	arr := make([]interface{}, 0, v.Len())
	limit := v.Len()
	if limit > maxElem {
		limit = maxElem
	}
	for i := 0; i < limit; i++ {
		elem := v.Index(i)
		if ds.cs.SkipNilValues && (elem.Kind() == reflect.Ptr || elem.Kind() == reflect.Interface || elem.Kind() == reflect.Map || elem.Kind() == reflect.Slice) && elem.IsNil() {
			continue
		}
		arr = append(arr, ds.dumpWithDepth(elem, depth+1))
	}
	if v.Len() > maxElem {
		arr = append(arr, fmt.Sprintf("...(truncated, %d items omitted, total %d, type %s)", v.Len()-maxElem, v.Len(), v.Type().String()))
	}
	return arr
}

type interfaceDumper struct{}

func (d *interfaceDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	if v.IsNil() {
		return nil
	}
	// 接口拆箱不增加深度，保证业务结构的层级更直观
	return ds.dumpWithDepth(v.Elem(), depth)
}

// safeDumpBytes 统一的安全字节数组/切片转储函数
func safeDumpBytes(v reflect.Value, typeName string) interface{} {
	// 基础有效性检查
	if !v.IsValid() {
		return "[]byte{<invalid_value>}"
	}

	// 检查类型是否正确
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Sprintf("[]byte{<wrong_type_%s>}", v.Kind().String())
	}

	// 检查元素类型
	if v.Type().Elem().Kind() != reflect.Uint8 {
		return fmt.Sprintf("[]byte{<wrong_elem_type_%s>}", v.Type().Elem().Kind().String())
	}

	// 检查空指针（针对切片）
	if v.Kind() == reflect.Slice {
		if v.Len() > 0 && v.Pointer() == 0 {
			return fmt.Sprintf("[]byte{<nil_data_len_%d>}", v.Len())
		}
		if v.IsNil() {
			return "[]byte{<nil_slice>}"
		}
	}

	// 尝试获取字节数据
	b, err := safeGetBytes(v)
	if err != nil {
		return fmt.Sprintf("[]byte{<access_failed_len_%d_err_%s>}", v.Len(), err.Error())
	}

	totalLen := len(b)
	if totalLen == 0 {
		return ""
	}

	// 安全查找末尾零字节
	end := safeFindNonZeroEnd(b, totalLen)
	zeroCount := totalLen - end

	// 安全处理内容
	var outStr string
	if end > 0 {
		outStr = safeFormatBytes(b[:end], typeName, totalLen)
	} else {
		outStr = ""
	}

	if zeroCount > 0 {
		outStr += fmt.Sprintf("...(truncated, %d zero bytes omitted, total %d bytes)", zeroCount, totalLen)
	}
	return outStr
}

func safeGetBytes(v reflect.Value) ([]byte, error) {
	// 预检查
	if !v.IsValid() {
		return nil, fmt.Errorf("invalid reflect value")
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil, fmt.Errorf("not a slice or array")
	}

	if v.Type().Elem().Kind() != reflect.Uint8 {
		return nil, fmt.Errorf("not a byte slice/array")
	}

	// 检查长度
	if v.Len() == 0 {
		return []byte{}, nil
	}

	// 对于切片，检查指针
	if v.Kind() == reflect.Slice && v.Pointer() == 0 {
		return nil, fmt.Errorf("nil slice pointer with non-zero length")
	}

	var result []byte
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic accessing bytes: %v", r)
				result = nil
			}
		}()

		// 尝试获取字节数据
		result = v.Bytes()

		// 额外验证：确保返回的切片长度正确
		if result != nil && len(result) != v.Len() {
			err = fmt.Errorf("length mismatch: expected %d, got %d", v.Len(), len(result))
			result = nil
		}
	}()

	return result, err
}

func safeFindNonZeroEnd(b []byte, totalLen int) int {
	// 安全性检查：确保切片有效且不为空
	if len(b) == 0 || totalLen <= 0 {
		return 0
	}

	// 确保totalLen不超过实际长度
	if totalLen > len(b) {
		totalLen = len(b)
	}

	end := totalLen

	// 使用更安全的方式访问字节数组
	func() {
		defer func() {
			if r := recover(); r != nil {
				// 发生panic时保持totalLen，不进行任何截断
				end = totalLen
			}
		}()

		// 额外的边界检查
		if totalLen > 0 && totalLen <= len(b) {
			// 从后往前查找非零字节，使用更安全的索引访问
			for i := totalLen - 1; i >= 0; i-- {
				if i < len(b) && b[i] != 0 {
					end = i + 1
					break
				}
				if i == 0 {
					// 所有字节都是0
					end = 0
					break
				}
			}
		}
	}()

	return end
}

func safeFormatBytes(b []byte, typeName string, totalLen int) string {
	var result string
	func() {
		defer func() {
			if recover() != nil {
				result = fmt.Sprintf("<unsafe_byte_%s_len_%d>", typeName, totalLen)
			}
		}()

		if len(b) == 0 {
			result = ""
			return
		}

		if isPrintableOrControlASCII(b) || isJSON(b) {
			result = string(b)
		} else {
			result = hex.EncodeToString(b)
		}
	}()
	return result
}
