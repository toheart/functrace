package objectdump

import (
	"fmt"
	"reflect"
)

// --- Dumper Implementations for Basic Types ---

type invalidDumper struct{}

func (d *invalidDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	return "invalid"
}

type boolDumper struct{}

func (d *boolDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	return v.Bool()
}

type intDumper struct{}

func (d *intDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	return v.Int()
}

type uintDumper struct{}

func (d *uintDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	return v.Uint()
}

type complexDumper struct{}

func (d *complexDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	return fmt.Sprintf("%v", v.Complex())
}

type stringDumper struct{}

func (d *stringDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	return v.String()
}

type chanDumper struct{}

func (d *chanDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	if v.IsNil() {
		return nil
	}
	return fmt.Sprintf("<chan %s %#x>", v.Type().String(), v.Pointer())
}

type funcDumper struct{}

func (d *funcDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	if v.IsNil() {
		return nil
	}
	return fmt.Sprintf("<func %s %#x>", v.Type().String(), v.Pointer())
}

type unsafePointerDumper struct{}

func (d *unsafePointerDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	if v.IsNil() {
		return nil
	}
	return fmt.Sprintf("<unsafe.Pointer %#x>", v.Pointer())
}
