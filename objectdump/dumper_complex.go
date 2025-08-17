package objectdump

import (
	"encoding/hex"
	"fmt"
	"reflect"
)

// --- Complex Type Dumpers ---

type ptrDumper struct{}

func (d *ptrDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
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
	// 解引用指针不计入逻辑深度，避免多层 *T 过早触发 MaxDepth
	return ds.dumpWithDepth(v.Elem(), depth)
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
	b := v.Bytes()
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
	return outStr
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
	b := v.Bytes()
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
	return outStr
}

func (d *arrayDumper) dumpGenericArray(ds *dumpState, v reflect.Value, depth int) interface{} {
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
