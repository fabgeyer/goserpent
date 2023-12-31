package main

import (
	"fmt"
)

type GoType string

func (g GoType) PyArgFormat() string {
	// Returns the format used for PyArg_ParseTupleAndKeywords()
	// https://docs.python.org/3/c-api/arg.html

	switch g {
	case "bool":
		return "p"
	case "int":
		return "i"
	case "int32":
		return "l"
	case "uint32":
		return "k"
	case "int64":
		return "L"
	case "uint64":
		return "K"
	case "float32":
		return "f"
	case "float64":
		return "d"
	case "string":
		return "s"
	case "*C.PyObject":
		return "O"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", g))
	}
}

func (g GoType) GoCType() string {
	switch g {
	case "int", "bool":
		return "C.int"
	case "int16":
		return "C.int16_t"
	case "uint16":
		return "C.uint16_t"
	case "int32":
		return "C.int32_t"
	case "uint32":
		return "C.uint32_t"
	case "int64":
		return "C.int64_t"
	case "uint64":
		return "C.uint64_t"
	case "float32":
		return "C.float"
	case "float64":
		return "C.double"
	case "string":
		return "*C.char"
	case "*C.PyObject":
		return "*C.PyObject"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", g))
	}
}

func (g GoType) CPtrType() string {
	switch g {
	case "int", "bool":
		return "int *"
	case "int32":
		return "int32_t *"
	case "int64":
		return "int64_t *"
	case "uint32":
		return "uint32_t *"
	case "uint64":
		return "uint64_t *"
	case "float32":
		return "float *"
	case "float64":
		return "double *"
	case "string":
		return "char **"
	case "*C.PyObject":
		return "PyObject **"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", g))
	}
}

func (g GoType) PythonType() string {
	switch g {
	case "":
		return "NoneType"
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return "int"
	case "string":
		return "str"
	case "bool":
		return "bool"
	case "*C.PyObject":
		return "object"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", g))
	}
}
