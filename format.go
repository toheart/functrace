package functrace

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"runtime"
)

// prepareParamsOutput 准备参数输出
func prepareParamsOutput(params []interface{}) []*TraceParams {
	var traceParams []*TraceParams

	// 如果没有参数，返回一个特殊标记
	if len(params) == 0 {
		traceParams = append(traceParams, &TraceParams{
			Pos:   -1,
			Param: "No parameters",
		})
		return traceParams
	}

	// 处理参数
	for i, item := range params {
		traceParams = append(traceParams, &TraceParams{
			Pos:   i,
			Param: formatParam(i, item),
		})
	}

	return traceParams
}

// formatParam 格式化单个参数
func formatParam(index int, item interface{}) string {
	val := reflect.ValueOf(item)
	if !val.IsValid() {
		return fmt.Sprintf("#%d: nil", index)
	}

	// 获取参数类型名称
	typeName := val.Type().String()

	// 使用 Output 函数获取格式化后的值
	formattedValue := Output(item, val)

	return fmt.Sprintf("#%d(%s): %s", index, typeName, formattedValue)
}

// Output 根据类型格式化跟踪参数以便记录
func Output(item interface{}, val reflect.Value) string {
	if !val.IsValid() {
		return "nil"
	}

	switch val.Kind() {
	case reflect.Func:
		return formatFunc(val)
	case reflect.String:
		return formatString(val)
	case reflect.Ptr:
		return formatPtr(item, val)
	case reflect.Interface:
		return formatInterface(item, val)
	case reflect.Struct:
		return formatStruct(item, val)
	case reflect.Map:
		return formatMap(item, val)
	case reflect.Array:
		return formatSliceOrArray(item, val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return formatInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return formatUint(val)
	case reflect.Float32, reflect.Float64:
		return formatFloat(val)
	case reflect.Bool:
		return formatBool(val)
	case reflect.Chan:
		return formatChan(val)
	case reflect.Complex64, reflect.Complex128:
		return formatComplex(val)
	case reflect.Slice:
		if val.Type().Elem().Kind() == reflect.Uint8 {
			return base64.StdEncoding.EncodeToString(val.Interface().([]byte))
		} else {
			return formatSliceOrArray(item, val)
		}
	default:
		return formatDefault(item, val)
	}
}

// 以下是 Output 函数的辅助函数

func formatFunc(val reflect.Value) string {
	return fmt.Sprintf("func(%s)", runtime.FuncForPC(val.Pointer()).Name())
}

func formatString(val reflect.Value) string {
	return fmt.Sprintf("\"%s\"", val.String())
}

func formatPtr(item interface{}, val reflect.Value) string {
	if val.IsNil() {
		return "nil"
	}
	return fmt.Sprintf("&%s", Output(val.Elem().Interface(), val.Elem()))
}

func formatInterface(item interface{}, val reflect.Value) string {
	if val.IsNil() {
		return "nil"
	}
	return Output(val.Elem().Interface(), val.Elem())
}

func formatStruct(item interface{}, val reflect.Value) string {
	typeName := val.Type().String()
	return fmt.Sprintf("%s: %+v", typeName, item)
}

func formatMap(item interface{}, val reflect.Value) string {
	if val.IsNil() {
		return "nil"
	}
	typeName := val.Type().String()
	return fmt.Sprintf("%s: %+v", typeName, item)
}

func formatSliceOrArray(item interface{}, val reflect.Value) string {
	if val.Kind() == reflect.Slice && val.IsNil() {
		return "nil"
	}
	typeName := val.Type().String()
	return fmt.Sprintf("%s: %+v", typeName, item)
}

func formatInt(val reflect.Value) string {
	return fmt.Sprintf("%d", val.Int())
}

func formatUint(val reflect.Value) string {
	return fmt.Sprintf("%d", val.Uint())
}

func formatFloat(val reflect.Value) string {
	return fmt.Sprintf("%.4f", val.Float())
}

func formatBool(val reflect.Value) string {
	return fmt.Sprintf("%v", val.Bool())
}

func formatChan(val reflect.Value) string {
	if val.IsNil() {
		return "nil"
	}
	typeName := val.Type().String()
	return fmt.Sprintf("%s: (chan)", typeName)
}

func formatComplex(val reflect.Value) string {
	return fmt.Sprintf("%v", val.Complex())
}

func formatDefault(item interface{}, val reflect.Value) string {
	typeName := val.Type().String()
	return fmt.Sprintf("%s: %+v", typeName, item)
}
