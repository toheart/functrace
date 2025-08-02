package objectdump

import (
	"bytes"
	"fmt"
	"reflect"
	"unsafe"
)

// isDeepType checks if a kind is a deep type that can be recursed into.
func isDeepType(k reflect.Kind) bool {
	switch k {
	case reflect.Ptr, reflect.Struct, reflect.Map, reflect.Slice, reflect.Array, reflect.Interface:
		return true
	default:
		return false
	}
}

// truncatedContainerInfo creates a string for truncated container types.
func truncatedContainerInfo(typ string, depth, maxDepth, l, c int) string {
	if l > 0 {
		return fmt.Sprintf("<truncated: type=%s, len=%d, cap=%d, depth=%d/%d>", typ, l, c, depth, maxDepth)
	}
	return fmt.Sprintf("<truncated: type=%s, depth=%d/%d>", typ, depth, maxDepth)
}

// isPrintableOrControlASCII checks if a byte slice is printable ASCII.
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

// isJSON checks if a byte slice looks like JSON.
func isJSON(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	trimmed := bytes.TrimSpace(b)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

// bypassUnsafeReflectValue uses unsafe.Pointer to read unexported fields.
func bypassUnsafeReflectValue(field reflect.StructField, v reflect.Value) reflect.Value {
	if !v.CanAddr() {
		return reflect.ValueOf("<unexported, not addressable>")
	}
	ptr := unsafe.Pointer(v.UnsafeAddr() + field.Offset)
	fv := reflect.NewAt(field.Type, ptr).Elem()
	return fv
}
