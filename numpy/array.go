package numpy

/*
#cgo pkg-config: python3 python3-embed
#include <Python.h>
#define NPY_NO_DEPRECATED_API NPY_1_7_API_VERSION
#include <numpy/ndarrayobject.h>

int PyArraySIZE(PyArrayObject *obj) {
	return PyArray_SIZE(obj);
}

char* PyArrayBYTES(PyArrayObject *obj, uint64_t *itemsize) {
	if (obj == NULL) { return NULL; }
	if (itemsize != NULL) {
		*itemsize = PyArray_ITEMSIZE(obj);
	}
	return PyArray_BYTES(obj);
}

void *PyArrayGETPTR1(PyArrayObject *obj, int i) {
	return PyArray_GETPTR1(obj, i);
}

void *PyArrayGETPTR2(PyArrayObject *obj, int i, int j) {
	return PyArray_GETPTR2(obj, i, j);
}

void *PyArrayGETPTR3(PyArrayObject *obj, int i, int j, int k) {
	return PyArray_GETPTR3(obj, i, j, k);
}

void *PyArrayGETPTR4(PyArrayObject *obj, int i, int j, int k, int l) {
	return PyArray_GETPTR4(obj, i, j, k, l);
}

void *PyArrayGETPTRN(PyArrayObject *obj, const int64_t *idxs) {
	int n = PyArray_NDIM(obj);
	npy_intp sz = PyArray_ITEMSIZE(obj);
	char *ptr = PyArray_BYTES(obj);
	for (int i = 0; i < n; i++) {
		ptr += (idxs[i])*PyArray_STRIDES(obj)[i];
	}
	return (void*)ptr;
}
*/
import "C"

import (
	"fmt"
	"iter"
	"unsafe"
)

//go:generate stringer -type=NumpyType
type NumpyType int

// Constants from numpy/ndarraytypes.h enum NPY_TYPES
const (
	NPY_BOOL        NumpyType = C.NPY_BOOL
	NPY_BYTE        NumpyType = C.NPY_BYTE
	NPY_UBYTE       NumpyType = C.NPY_UBYTE
	NPY_SHORT       NumpyType = C.NPY_SHORT
	NPY_USHORT      NumpyType = C.NPY_USHORT
	NPY_INT         NumpyType = C.NPY_INT
	NPY_UINT        NumpyType = C.NPY_UINT
	NPY_LONG        NumpyType = C.NPY_LONG
	NPY_ULONG       NumpyType = C.NPY_ULONG
	NPY_LONGLONG    NumpyType = C.NPY_LONGLONG
	NPY_ULONGLONG   NumpyType = C.NPY_ULONGLONG
	NPY_FLOAT       NumpyType = C.NPY_FLOAT
	NPY_DOUBLE      NumpyType = C.NPY_DOUBLE
	NPY_LONGDOUBLE  NumpyType = C.NPY_LONGDOUBLE
	NPY_CFLOAT      NumpyType = C.NPY_CFLOAT
	NPY_CDOUBLE     NumpyType = C.NPY_CDOUBLE
	NPY_CLONGDOUBLE NumpyType = C.NPY_CLONGDOUBLE
	NPY_OBJECT      NumpyType = C.NPY_OBJECT
	NPY_STRING      NumpyType = C.NPY_STRING
	NPY_UNICODE     NumpyType = C.NPY_UNICODE
	NPY_VOID        NumpyType = C.NPY_VOID
	NPY_DATETIME    NumpyType = C.NPY_DATETIME
	NPY_TIMEDELTA   NumpyType = C.NPY_TIMEDELTA
	NPY_HALF        NumpyType = C.NPY_HALF
)

type Array struct {
	obj   *C.PyArrayObject
	dtype NumpyType
	dims  int
}

func AsArray(ptr unsafe.Pointer) *Array {
	obj := (*C.PyArrayObject)(ptr)
	return &Array{
		obj:   obj,
		dtype: NumpyType(C.PyArray_TYPE(obj)),
		dims:  int(C.PyArray_NDIM(obj)),
	}
}

func (a *Array) PyObject() unsafe.Pointer {
	return unsafe.Pointer(a.obj)
}

func (a *Array) PyArrayObject() *C.PyArrayObject {
	return a.obj
}

// Return the (builtin) typenumber for the elements of this array.
func (a *Array) Type() NumpyType {
	return a.dtype
}

// The number of dimensions in the array.
func (a *Array) Dims() int {
	a.dims = int(C.PyArray_NDIM(a.obj))
	return a.dims
}

// Returns a slice of the dimensions/shape of the array. The number of elements matches the number of dimensions of the array. Can return nil for 0-dimensional arrays.
func (a *Array) Shape() []int {
	res := make([]int, a.Dims())
	shape := unsafe.Slice(C.PyArray_SHAPE(a.obj), 4)
	for i := range res {
		res[i] = int(shape[i])
	}
	return res
}

// Returns the total size (in number of elements) of the array.
func (a *Array) Size() int {
	return int(C.PyArraySIZE(a.obj))
}

func (a *Array) toValue(ptr unsafe.Pointer) interface{} {
	switch a.dtype {
	case NPY_FLOAT:
		return *(*float32)(ptr)
	case NPY_DOUBLE:
		return *(*float64)(ptr)
	case NPY_SHORT:
		return int16(*(*C.short)(ptr))
	case NPY_INT:
		return int(*(*C.int)(ptr))
	case NPY_UINT:
		return uint(*(*C.uint)(ptr))
	case NPY_LONG:
		return int(*(*C.long)(ptr))
	case NPY_LONGLONG:
		return int64(*(*C.longlong)(ptr))
	case NPY_ULONG:
		return uint(*(*C.ulong)(ptr))
	case NPY_ULONGLONG:
		return uint64(*(*C.longlong)(ptr))
	default:
		panic(fmt.Sprintf("unsupported type %v", a.dtype))
	}
}

func (a *Array) getPtr(idxs []int) unsafe.Pointer {
	if len(idxs) != a.dims {
		panic("invalid indexing")
	}

	switch a.dims {
	case 1:
		return unsafe.Pointer(C.PyArrayGETPTR1(a.obj, C.int(idxs[0])))
	case 2:
		return unsafe.Pointer(C.PyArrayGETPTR2(a.obj, C.int(idxs[0]), C.int(idxs[1])))
	case 3:
		return unsafe.Pointer(C.PyArrayGETPTR3(a.obj, C.int(idxs[0]), C.int(idxs[1]), C.int(idxs[2])))
	case 4:
		return unsafe.Pointer(C.PyArrayGETPTR4(a.obj, C.int(idxs[0]), C.int(idxs[1]), C.int(idxs[2]), C.int(idxs[4])))
	default:
		return unsafe.Pointer(C.PyArrayGETPTRN(a.obj, (*C.int64_t)(unsafe.Pointer(&idxs[0]))))
	}
}

func (a *Array) At(idxs ...int) interface{} {
	return a.toValue(a.getPtr(idxs))
}

func (a *Array) SetAt(v interface{}, idxs ...int) {
	ptr := a.getPtr(idxs)
	switch a.dtype {
	case NPY_FLOAT:
		*(*float32)(ptr) = v.(float32)
	case NPY_DOUBLE:
		*(*float64)(ptr) = v.(float64)
	case NPY_SHORT:
		*(*C.short)(ptr) = C.short(v.(int16))
	case NPY_INT:
		*(*C.int)(ptr) = C.int(v.(int))
	case NPY_UINT:
		*(*C.uint)(ptr) = C.uint(v.(uint))
	case NPY_LONG:
		*(*C.long)(ptr) = C.long(v.(int))
	default:
		panic(fmt.Sprintf("unsupported type %v", a.dtype))
	}
}

func (a *Array) Values() iter.Seq[interface{}] {
	nitems := 1
	for _, v := range a.Shape() {
		nitems *= v
	}

	bytes, itemsize := a.Bytes()
	return func(yield func(interface{}) bool) {
		for i := range uint64(nitems) {
			if !yield(a.toValue(unsafe.Add(bytes, i*itemsize))) {
				return
			}
		}
	}
}

func (a *Array) IndexedValues() iter.Seq2[[]int, interface{}] {
	shape := a.Shape()
	remaining := 1
	for _, v := range shape {
		remaining *= v
	}

	coords := make([]int, len(shape))

	return func(yield func([]int, interface{}) bool) {
		for remaining > 0 {
			if !yield(coords, a.At(coords...)) {
				return
			}

			for k := len(coords) - 1; k >= 0; k-- {
				if coords[k]+1 < shape[k] {
					coords[k] += 1
					break
				} else {
					coords[k] = 0
				}
			}
			remaining--
		}
	}
}

func (a *Array) Bytes() (unsafe.Pointer, uint64) {
	var itemsize uint64
	return unsafe.Pointer(C.PyArrayBYTES(a.obj, (*C.uint64_t)(&itemsize))), itemsize
}

func Values[T any](a *Array) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range a.Values() {
			if !yield(v.(T)) {
				return
			}
		}
	}
}

func IndexedValues[T any](a *Array) iter.Seq2[[]int, T] {
	return func(yield func([]int, T) bool) {
		for k, v := range a.IndexedValues() {
			if !yield(k, v.(T)) {
				return
			}
		}
	}
}
