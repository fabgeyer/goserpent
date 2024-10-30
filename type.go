package main

import (
	"fmt"
	"go/ast"

	"github.com/rs/zerolog/log"
)

//go:generate stringer -type=Kind
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
	Byte
	ByteArray
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
	case "complex64":
		return Complex64
	case "complex128":
		return Complex128
	case "string":
		return String
	case "byte":
		return Byte
	}
	log.Fatal().Caller().Msgf("Type '%s' not supported!", v)
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

		switch elt.T {
		case Byte:
			// Note: Handle byte slices as its own type as Python has its own PyBytes
			return &GoType{
				T:      ByteArray,
				GoRepr: GetSourceString(context, expr),
			}, nil

		default:
			return &GoType{
				T:             Slice,
				SliceElemType: elt,
				GoRepr:        GetSourceString(context, expr),
			}, nil
		}

	case *ast.MapType:
		mapKeyType, err := AsGoType(v.Key, context)
		if err != nil {
			return nil, err
		}
		mapValType, err := AsGoType(v.Value, context)
		if err != nil {
			return nil, err
		}
		return &GoType{
			T:          Map,
			MapKeyType: mapKeyType,
			MapValType: mapValType,
			GoRepr:     GetSourceString(context, expr),
		}, nil

	default:
		return nil, fmt.Errorf("Type '%s' (%T) not supported!", GetSourceString(context, expr), expr)
	}
}

func (g *GoType) Unsupported() {
	if g.GoRepr == "" {
		log.Fatal().Caller(1).Msgf("Type '%+v' not supported", g)
	} else {
		log.Fatal().Caller(1).Msgf("Type '%s' not supported", g.GoRepr)
	}
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
	case Complex64, Complex128:
		return "D"
	case String:
		return "s"
	case Map, Slice, CPyObjectPointer:
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
	case Complex64, Complex128:
		return "C.Py_complex"
	case String:
		return "*C.char"
	case Map, Slice, CPyObjectPointer:
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
	case Complex64, Complex128:
		return "Py_complex *"
	case String:
		return "char **"
	case Map, Slice, CPyObjectPointer:
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
	case Complex64, Complex128:
		return "complex"
	case Slice:
		return fmt.Sprintf("List[%s]", g.SliceElemType.PythonTypeHint())
	case ByteArray:
		return "bytes"
	default:
		g.Unsupported()
	}
	panic("")
}

func (g *GoType) GoPyReturn(varname string) string {
	switch g.T {
	case None: // Equivalent of Python's None
		return "return C.Py_None"
	case CPyObjectPointer:
		return fmt.Sprintf("return %s", varname)
	case Bool:
		return fmt.Sprintf("return asPyBool(%s)", varname)
	case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64:
		return fmt.Sprintf("return asPyLong(%s)", varname)
	case Float32, Float64:
		return fmt.Sprintf("return asPyFloat(%s)", varname)
	case Complex64:
		return fmt.Sprintf("return goComplex64AsPyComplex(%s)", varname)
	case Complex128:
		return fmt.Sprintf("return goComplex128AsPyComplex(%s)", varname)
	case String:
		return fmt.Sprintf("return asPyString(%s)", varname)
	case Error:
		return fmt.Sprintf("return asPyError(%s)", varname)
	case Pointer:
		return fmt.Sprintf("return %sToPyObject(%s)", g.GoRepr, varname)
	case Map:
		return fmt.Sprintf("return asPyDict(%s, %s, %s)", varname,
			g.MapKeyType.GoPyReturnLambda(),
			g.MapValType.GoPyReturnLambda())
	case Slice:
		return fmt.Sprintf("return asPyList(%s, %s)", varname, g.SliceElemType.GoPyReturnLambda())
	case ByteArray:
		return fmt.Sprintf("return asPyBytes(%s)", varname)
	default:
		g.Unsupported()
	}
	panic("")
}

func (g *GoType) GoPyReturnLambda() string {
	switch g.T {
	case Pointer:
		return fmt.Sprintf("func(v *%s) *C.PyObject { %s }", g.GoRepr, g.GoPyReturn("v"))
	default:
		return fmt.Sprintf("func(v %s) *C.PyObject { %s }", g.GoRepr, g.GoPyReturn("v"))
	}
}

func (g *GoType) CToGoFunction(varname string) string {
	switch g.T {
	case CPyObjectPointer:
		return varname
	case Bool:
		return fmt.Sprintf("asGoBool(%s)", varname)
	case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64:
		return fmt.Sprintf("%s(%s)", g.GoRepr, varname)
	case Complex64:
		return fmt.Sprintf("asGoComplex64(%s)", varname)
	case Complex128:
		return fmt.Sprintf("asGoComplex128(%s)", varname)
	case String:
		return fmt.Sprintf("C.GoString(%s)", varname)
	case Slice:
		return fmt.Sprintf("asGoSlice(%s, %s)", varname,
			g.SliceElemType.CPyObjectToGoLambda())
	case Map:
		return fmt.Sprintf("asGoMap(%s, %s, %s)", varname,
			g.MapKeyType.CPyObjectToGoLambda(),
			g.MapValType.CPyObjectToGoLambda())
	}
	g.Unsupported()
	panic("")
}

func (g *GoType) CToGoLambdaFunction() string {
	return fmt.Sprintf("func(o %s) %s { return %s }", g.GoCType(), g.GoRepr, g.CToGoFunction("o"))
}

func (g *GoType) CPyObjectToGo(cPyObjectVarName string) string {
	switch g.T {
	case String:
		return fmt.Sprintf("pyObjectAsGoString(%s)", cPyObjectVarName)
	case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64:
		return fmt.Sprintf("asGoInt[%s](%s)", g.GoRepr, cPyObjectVarName)
	case Float32, Float64:
		return fmt.Sprintf("asGoFloat[%s](%s)", g.GoRepr, cPyObjectVarName)
	}
	g.Unsupported()
	panic("")
}

func (g *GoType) CPyObjectToGoLambda() string {
	switch g.T {
	case String:
		return "pyObjectAsGoString"
	case Float32, Float64:
		return fmt.Sprintf("asGoFloat[%s]", g.GoRepr)
	case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64:
		return fmt.Sprintf("asGoInt[%s]", g.GoRepr)
	default:
		g.Unsupported()
	}
	panic("")
}

func (g *GoType) IsNotNone() bool {
	return g.T != None
}
