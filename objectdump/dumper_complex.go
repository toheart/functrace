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
	m := make(map[string]interface{})
	keys := v.MapKeys()
	for _, k := range keys {
		val := v.MapIndex(k)
		if ds.cs.SkipNilValues && (val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface || val.Kind() == reflect.Map || val.Kind() == reflect.Slice) && val.IsNil() {
			continue
		}
		m[fmt.Sprint(k.Interface())] = ds.dumpWithDepth(val, depth+1)
	}
	return m
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
