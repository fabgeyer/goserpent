//go:build pythonnumpy

package main

import (
	"fmt"

	"github.com/fabgeyer/goserpent/numpy"
)

// go:pyexport
func PrintDescr(obj *numpy.Array) {
	fmt.Printf("type=%v dims=%d shape=%v\n", obj.Type(), obj.Dims(), obj.Shape())
}

// go:pyexport
func PrintValues(obj *numpy.Array) {
	for val := range obj.Values() {
		fmt.Printf("%v\n", val)
	}
}

// go:pyexport
func PrintIndexedValues(obj *numpy.Array) {
	for idxs, val := range obj.IndexedValues() {
		fmt.Printf("%v = %v\n", idxs, val)
	}
}

// go:pyexport
func AddIntValue(obj *numpy.Array, v int) {
	switch obj.Type() {
	case numpy.NPY_SHORT:
		for idxs, val := range numpy.IndexedValues[int16](obj) {
			obj.SetAt(val+int16(v), idxs...)
		}

	case numpy.NPY_LONG, numpy.NPY_INT:
		for idxs, val := range numpy.IndexedValues[int](obj) {
			obj.SetAt(val+v, idxs...)
		}

	case numpy.NPY_FLOAT:
		for idxs, val := range numpy.IndexedValues[float32](obj) {
			obj.SetAt(val+float32(v), idxs...)
		}

	case numpy.NPY_DOUBLE:
		for idxs, val := range numpy.IndexedValues[float64](obj) {
			obj.SetAt(val+float64(v), idxs...)
		}

	default:
		panic("invalid dtype")
	}
}
