package main

import (
	"fmt"
	"go/ast"

	"github.com/rs/zerolog/log"
)

type Kind uint

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Pointer
	Slice
	String
	Struct
	UnsafePointer
	None
	Error
	CPyObjectPointer
)

type GoType struct {
	T             Kind
	SliceElemType *GoType
	MapKeyType    *GoType
	MapValType    *GoType
	PointerTo     *GoType
	GoRepr        string
}

func ToKind(v string) Kind {
	switch v {
	case "int":
		return Int
	case "int8":
		return Int8
	case "int16":
		return Int16
	case "int32":
		return Int32
	case "int64":
		return Int64
	case "uint":
		return Uint
	case "uint8":
		return Uint8
	case "uint16":
		return Uint16
	case "uint32":
		return Uint32
	case "uint64":
		return Uint64
	case "bool":
		return Bool
	case "error":
		return Error
	case "float32":
		return Float32
	case "float64":
		return Float64
	case "string":
		return String
	default:
		log.Fatal().Caller().Msgf("Type '%s' not supported!", v)
	}
	panic("")
}

func IsCPyObjectPtr(expr ast.Expr) bool {
	sta, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := sta.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ide, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if ide.Name == "C" && sel.Sel.Name == "PyObject" {
		return true
	}
	return false
}

func AsGoType(expr ast.Expr, context []byte) (*GoType, error) {
	switch v := expr.(type) {
	case *ast.Ident:
		return &GoType{
			T:      ToKind(v.Name),
			GoRepr: GetSourceString(context, expr),
		}, nil

	case *ast.StarExpr:
		if IsCPyObjectPtr(v) {
			return &GoType{T: CPyObjectPointer}, nil

		} else {
			return &GoType{
				T:      Pointer,
				GoRepr: GetSourceString(context, expr)[1:],
			}, nil
		}

	case *ast.ArrayType:
		elt, err := AsGoType(v.Elt, context)
		if err != nil {
			return nil, err
		}
		return &GoType{T: Slice, SliceElemType: elt}, nil

	case *ast.MapType:
		mapKeyType, err := AsGoType(v.Key, context)
		if err != nil {
			return nil, err
		}
		mapValType, err := AsGoType(v.Value, context)
		if err != nil {
			return nil, err
		}
		return &GoType{T: Map, MapKeyType: mapKeyType, MapValType: mapValType}, nil

	default:
		return nil, fmt.Errorf("Type '%s' (%T) not supported!", GetSourceString(context, expr), expr)
	}
}

func (g *GoType) Unsupported() {
	log.Fatal().Caller(1).Msgf("Type '%s' not supported", g)
}

func (g *GoType) PyArgFormat() string {
	// Returns the format used for PyArg_ParseTupleAndKeywords()
	// https://docs.python.org/3/c-api/arg.html

	switch g.T {
	case Bool:
		return "p"
	case Int:
		return "i"
	case Int32:
		return "l"
	case Uint32:
		return "k"
	case Int64:
		return "L"
	case Uint64:
		return "K"
	case Float32:
		return "f"
	case Float64:
		return "d"
	case String:
		return "s"
	case Map, CPyObjectPointer:
		return "O"
	}
	g.Unsupported()
	panic("")
}

func (g *GoType) GoCType() string {
	switch g.T {
	case Int, Bool:
		return "C.int"
	case Int8:
		return "C.int8_t"
	case Uint8:
		return "C.uint8_t"
	case Int16:
		return "C.int16_t"
	case Uint16:
		return "C.uint16_t"
	case Int32:
		return "C.int32_t"
	case Uint32:
		return "C.uint32_t"
	case Int64:
		return "C.int64_t"
	case Uint64:
		return "C.uint64_t"
	case Float32:
		return "C.float"
	case Float64:
		return "C.double"
	case String:
		return "*C.char"
	case Map, CPyObjectPointer:
		return "*C.PyObject"
	}
	g.Unsupported()
	panic("")
}

func (g *GoType) CPtrType() string {
	switch g.T {
	case Int, Bool:
		return "int *"
	case Int32:
		return "int32_t *"
	case Int64:
		return "int64_t *"
	case Uint32:
		return "uint32_t *"
	case Uint64:
		return "uint64_t *"
	case Float32:
		return "float *"
	case Float64:
		return "double *"
	case String:
		return "char **"
	case Map, CPyObjectPointer:
		return "PyObject **"
	}
	g.Unsupported()
	panic("")
}

func (g *GoType) PythonTypeHint() string {
	switch g.T {
	case None:
		return "NoneType"
	case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64:
		return "int"
	case String:
		return "str"
	case Bool:
		return "bool"
	case Map:
		return fmt.Sprintf("Dict[%s, %s]", g.MapKeyType.PythonTypeHint(), g.MapValType.PythonTypeHint())
	case CPyObjectPointer:
		return "object"
	case Pointer:
		return g.GoRepr
	case Float32, Float64:
		return "float"
	case Slice:
		return fmt.Sprintf("List[%s]", g.SliceElemType.PythonTypeHint())
	}
	g.Unsupported()
	panic("")
}

func (g *GoType) GoPyReturn(result string) string {
	switch g.T {
	case None: // Equivalent of Python's None
		return "return C.Py_None"
	case CPyObjectPointer:
		return fmt.Sprintf("return %s", result)
	case Bool:
		return fmt.Sprintf("return asPyBool(%s)", result)
	case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64:
		return fmt.Sprintf("return asPyLong(%s)", result)
	case Float32, Float64:
		return fmt.Sprintf("return asPyFloat(%s)", result)
	case String:
		return fmt.Sprintf("return asPyString(%s)", result)
	case Error:
		return fmt.Sprintf("return asPyError(%s)", result)
	case Pointer:
		return fmt.Sprintf("return %sToPyObject(%s)", g.GoRepr, result)
	case Slice:
		switch g.SliceElemType.T {
		case Pointer:
			return fmt.Sprintf("return asPyList(%s, func(v *%s) *C.PyObject { %s })", result, g.SliceElemType.GoRepr, g.SliceElemType.GoPyReturn("v"))
		default:
			return fmt.Sprintf("return asPyList(%s, func(v %s) *C.PyObject { %s })", result, g.SliceElemType.GoRepr, g.SliceElemType.GoPyReturn("v"))
		}
	}
	g.Unsupported()
	panic("")
}

func (g *GoType) IsNotNone() bool {
	return g.T != None
}

func TplCPyObjectToGo(g *GoType, cPyObjectVarName string) string {
	switch g.T {
	case String:
		return fmt.Sprintf("pyObjectAsGoString(%s)", cPyObjectVarName)
	case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64:
		// https://docs.python.org/3/c-api/long.html
		return fmt.Sprintf("%s(C.PyLong_AsLong(%s))", g.GoRepr, cPyObjectVarName)
	case Float32, Float64:
		// https://docs.python.org/3/c-api/float.html
		return fmt.Sprintf("%s(C.PyFloat_AsDouble(%s))", g.GoRepr, cPyObjectVarName)
	}
	g.Unsupported()
	panic("")
}
