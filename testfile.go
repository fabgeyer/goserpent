//go:build python

package main

// #cgo pkg-config: python3 python3-embed
// #include <Python.h>
import "C"

import (
	"fmt"
)

// Automatically exported as it returns a *C.PyObject
func FunctionWithArgs(arg1, arg2 int, arg3 string) *C.PyObject {
	fmt.Printf("FunctionWithArgs(%d, %d, %s)\n", arg1, arg2, arg3)
	return C.Py_None
}

// Automatically exported as it returns a *C.PyObject
func BasicFunction() *C.PyObject {
	fmt.Println("BasicFunction()")
	return C.Py_None
}

// go:pyexport
func BasicFunctionWithError(arg int) (int, error) {
	fmt.Printf("BasicFunctionWithError(%d)\n", arg)
	if arg == 0 {
		return 0, fmt.Errorf("Invalid argument")
	}
	return arg, nil
}

// go:pyexport
func FunctionReturnBool(v bool) bool {
	fmt.Println("FunctionReturnBool()")
	return v
}

// go:pyexport
func FunctionReturnNone() {
	fmt.Println("FunctionReturnNone()")
}

// go:pyexport
func FunctionReturnInt(arg int) int {
	fmt.Printf("FunctionReturnInt(%d)\n", arg)
	return arg * 2
}

// go:pyexport
func FunctionReturnError(arg int) error {
	fmt.Printf("FunctionReturnError(%d)\n", arg)
	return fmt.Errorf("Example error")
}

// go:pyexport
func FunctionMapArgument(arg map[string]int) {
	fmt.Printf("FunctionMapArgument(%v)\n", arg)
}
