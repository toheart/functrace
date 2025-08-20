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
	"encoding/json"
	"fmt"
	"reflect"
)

// ConfigState houses the configuration options used by spew to format and
// display values. Only Indent, MaxDepth, and SkipNilValues are used in the current implementation.
type ConfigState struct {
	// Indent specifies the string to use for each indentation level.  The
	// global config instance that all top-level functions use set this to a
	// single space by default.  If you would like more indentation, you might
	// set this to a tab with "\t" or perhaps two spaces with "  ".
	Indent string

	// MaxDepth controls the maximum number of levels to descend into nested
	// data structures.  The default, 0, means there is no limit.
	//
	// NOTE: Circular data structures are properly detected, so it is not
	// necessary to set this value unless you specifically want to limit deeply
	// nested data structures.
	MaxDepth int

	// SkipNilValues specifies whether to skip nil values in the output.
	// When enabled, nil values will not be included in the JSON output.
	// This is useful for producing more concise JSON output.
	SkipNilValues bool

	// MaxElementsPerContainer specifies the maximum number of elements to include in a container before truncating.
	MaxElementsPerContainer int

	// AllowUnexported specifies whether to access and dump unexported fields
	// of structs. This requires using the unsafe package and is enabled by
	// default to maintain behavior, but can be disabled for safety.
	AllowUnexported bool

	// CompactLargeObjects specifies whether to compact large built-in objects
	// (like time.Time, net.Conn, etc.) into simple string representations.
	// When enabled, these objects will be shown as "<TypeName>" or their
	// string representation instead of full internal structure.
	// This is enabled by default to avoid extremely large outputs.
	CompactLargeObjects bool

	// ParseMapValues controls whether to deeply parse map entries.
	// When disabled (default), map and *map are summarized as type and length only.
	ParseMapValues bool
}

// Config is the active configuration of the top-level functions.
var Config = ConfigState{Indent: " ", MaxElementsPerContainer: 10, AllowUnexported: true, CompactLargeObjects: true, ParseMapValues: false}

func SetGlobalConfig(config *ConfigState) {
	Config = *config
}

// ToJSON returns a JSON string representation of the passed arguments using this configuration.
func (cs *ConfigState) ToJSON(a ...interface{}) string {
	if len(a) == 0 {
		return "{}"
	}

	d := getDumpState(cs)
	result := d.dump(reflect.ValueOf(a[0]))
	putDumpState(d)

	// Ensure the final output is always a JSON object for consistency.
	if _, ok := result.(map[string]interface{}); !ok {
		result = map[string]interface{}{"value": result}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal dumped object: %s"}`, err.Error())
	}
	return buf.String()
}
