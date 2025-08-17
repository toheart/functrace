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

package objectdump

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sync"
	"unicode/utf8"
)

var (
	// dumpStatePool is a pool of dumpState objects to reduce memory allocations.
	dumpStatePool = sync.Pool{
		New: func() interface{} {
			return &dumpState{
				pointers: make(map[uintptr]int),
			}
		},
	}
)

// dumpState contains the state information for a JSON dump operation.
type dumpState struct {
	depth    int
	pointers map[uintptr]int
	cs       *ConfigState
	visited  map[uintptr]int
}

// reset resets the dumpState to its initial state for reuse from the pool.
func (d *dumpState) reset() {
	d.depth = 0
	for k := range d.pointers {
		delete(d.pointers, k)
	}
	d.cs = nil
	d.visited = nil
}

// getDumpState retrieves a dumpState object from the pool and initializes it.
func getDumpState(cs *ConfigState) *dumpState {
	d := dumpStatePool.Get().(*dumpState)
	d.cs = cs
	d.depth = 0
	return d
}

// putDumpState returns a dumpState object to the pool.
func putDumpState(d *dumpState) {
	d.reset()
	dumpStatePool.Put(d)
}

// safeDump executes a dumper with panic recovery
func safeDump(dumper dumper, ds *dumpState, v reflect.Value, depth int) interface{} {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic for debugging
			fmt.Printf("Panic in dumper for kind %v: %v\n", v.Kind(), r)
		}
	}()

	return dumper.dump(ds, v, depth)
}

// dump is the main worker function that recursively dumps a value.
func (d *dumpState) dump(v reflect.Value) interface{} {
	return d.dumpWithDepth(v, -1)
}

func (d *dumpState) dumpWithDepth(v reflect.Value, depth int) interface{} {
	if !v.IsValid() {
		return "invalid"
	}

	// Centralized handling for large objects.
	if d.cs.CompactLargeObjects {
		valueToCheck := v
		if (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) && !v.IsNil() {
			valueToCheck = v.Elem()
		}
		if str, handled := handleLargeObject(valueToCheck); handled {
			return str
		}
	}

	// Depth truncation.
	// depth 表示“业务结构深度”：指针/接口解引用不会增加 depth，以免过早触发 MaxDepth。
	if d.cs.MaxDepth > 0 && depth > d.cs.MaxDepth && isDeepType(v.Kind()) {
		var l, c int
		switch v.Kind() {
		case reflect.Slice, reflect.Array:
			l, c = v.Len(), v.Cap()
		case reflect.Map:
			l = v.Len()
		case reflect.Interface:
			l, c = 1, 1
		}
		return truncatedContainerInfo(v.Type().String(), depth, d.cs.MaxDepth, l, c)
	}

	// Get the appropriate dumper strategy and execute it
	dumper := getDumper(v.Kind())
	if dumper == nil {
		return fmt.Sprintf("<nil dumper for kind: %v>", v.Kind())
	}

	// Execute dumper with panic recovery
	return safeDump(dumper, d, v, depth)
}

// Sdump returns a string with the passed arguments formatted as a JSON object.
// If the argument is not a struct or map, it will be wrapped in a {"value": ...} object.
func Sdump(a ...interface{}) string {
	if len(a) == 0 {
		return "{}"
	}
	// For functrace, we only ever dump one parameter at a time.
	arg := a[0]

	d := getDumpState(&Config)
	result := d.dump(reflect.ValueOf(arg))
	putDumpState(d)

	// Ensure the final output is always a JSON object for consistency.
	if _, ok := result.(map[string]interface{}); !ok {
		result = map[string]interface{}{"value": result}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		// As a fallback, return a JSON object with the error message.
		return fmt.Sprintf(`{"error": "failed to marshal dumped object: %s"}`, err.Error())
	}
	return buf.String()
}

// ToJSON returns a JSON string representation of the passed arguments.
func ToJSON(a ...interface{}) string {
	return Sdump(a...)
}

// SdumpJSON is an alias for ToJSON for backward compatibility.
func SdumpJSON(a ...interface{}) string {
	return ToJSON(a...)
}

// Fdump writes the JSON representation of the passed arguments to the writer.
func Fdump(w io.Writer, a ...interface{}) {
	result := Sdump(a...)
	w.Write([]byte(result))
}

// DumpToJSON returns a JSON string representation with custom configuration.
func DumpToJSON(a interface{}, cs *ConfigState) (string, error) {
	d := getDumpState(cs)
	result := d.dump(reflect.ValueOf(a))
	putDumpState(d)

	// Ensure the final output is always a JSON object for consistency.
	if _, ok := result.(map[string]interface{}); !ok {
		result = map[string]interface{}{"value": result}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		return "", fmt.Errorf("failed to marshal dumped object: %w", err)
	}
	return buf.String(), nil
}

// GetPoolStats returns the current memory pool statistics.
func GetPoolStats() (inUse, available int) {
	// This is a simplified implementation
	// In a real implementation, you might want to track actual pool usage
	return 0, 100 // Placeholder values
}

// isUTF8 checks if a byte slice is valid UTF-8.
func isUTF8(b []byte) bool {
	return utf8.Valid(b)
}

// getUint8String converts a uint8 slice to string, handling special cases.
func getUint8String(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	// Check if it's printable ASCII
	if isPrintableOrControlASCII(b) {
		return string(b)
	}

	// Check if it's valid UTF-8
	if isUTF8(b) {
		return string(b)
	}

	// Fall back to hex encoding
	return hex.EncodeToString(b)
}

// Dump returns a string representation of the passed arguments.
func Dump(a ...interface{}) string {
	return Sdump(a...)
}

// fdump is an alias for Fdump for backward compatibility.
func fdump(w io.Writer, a ...interface{}) {
	Fdump(w, a...)
}
