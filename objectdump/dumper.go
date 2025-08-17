package objectdump

import (
	"fmt"
	"reflect"
)

// dumper defines the interface for all type-specific dump strategies
type dumper interface {
	dump(ds *dumpState, v reflect.Value, depth int) interface{}
}

// dumperRegistry maps reflect.Kind to its corresponding dumper strategy
var dumperRegistry map[reflect.Kind]dumper

// init initializes the dumper registry with all available strategies
func init() {
	dumperRegistry = make(map[reflect.Kind]dumper)

	// Basic types
	dumperRegistry[reflect.Invalid] = &invalidDumper{}
	dumperRegistry[reflect.Bool] = &boolDumper{}
	dumperRegistry[reflect.Int] = &intDumper{}
	dumperRegistry[reflect.Int8] = &intDumper{}
	dumperRegistry[reflect.Int16] = &intDumper{}
	dumperRegistry[reflect.Int32] = &intDumper{}
	dumperRegistry[reflect.Int64] = &intDumper{}
	dumperRegistry[reflect.Uint] = &uintDumper{}
	dumperRegistry[reflect.Uint8] = &uintDumper{}
	dumperRegistry[reflect.Uint16] = &uintDumper{}
	dumperRegistry[reflect.Uint32] = &uintDumper{}
	dumperRegistry[reflect.Uint64] = &uintDumper{}
	dumperRegistry[reflect.Uintptr] = &uintDumper{}
	dumperRegistry[reflect.Float32] = &floatDumper{}
	dumperRegistry[reflect.Float64] = &floatDumper{}
	dumperRegistry[reflect.Complex64] = &complexDumper{}
	dumperRegistry[reflect.Complex128] = &complexDumper{}
	dumperRegistry[reflect.String] = &stringDumper{}
	dumperRegistry[reflect.Chan] = &chanDumper{}
	dumperRegistry[reflect.Func] = &funcDumper{}
	dumperRegistry[reflect.UnsafePointer] = &unsafePointerDumper{}

	// Complex types (will be implemented in separate files)
	dumperRegistry[reflect.Ptr] = &ptrDumper{}
	dumperRegistry[reflect.Struct] = &structDumper{}
	dumperRegistry[reflect.Map] = &mapDumper{}
	dumperRegistry[reflect.Slice] = &sliceDumper{}
	dumperRegistry[reflect.Array] = &arrayDumper{}
	dumperRegistry[reflect.Interface] = &interfaceDumper{}
}

// getDumper retrieves the appropriate dumper for a given reflect.Kind
func getDumper(kind reflect.Kind) dumper {
	if dumper, exists := dumperRegistry[kind]; exists && dumper != nil {
		return dumper
	}
	return &unsupportedDumper{}
}

// unsupportedDumper handles unsupported types
type unsupportedDumper struct{}

func (d *unsupportedDumper) dump(ds *dumpState, v reflect.Value, depth int) interface{} {
	return fmt.Sprintf("<unsupported %s>", v.Type().String())
}
