package objectdump

import (
	"math"
	"reflect"
)

type floatDumper struct{}

func (d *floatDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	f := v.Float()
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}
	return f
}
