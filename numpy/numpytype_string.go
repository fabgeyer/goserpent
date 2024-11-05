// Code generated by "stringer -type=NumpyType"; DO NOT EDIT.

package numpy

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[NPY_BOOL-0]
	_ = x[NPY_BYTE-1]
	_ = x[NPY_UBYTE-2]
	_ = x[NPY_SHORT-3]
	_ = x[NPY_USHORT-4]
	_ = x[NPY_INT-5]
	_ = x[NPY_UINT-6]
	_ = x[NPY_LONG-7]
	_ = x[NPY_ULONG-8]
	_ = x[NPY_LONGLONG-9]
	_ = x[NPY_ULONGLONG-10]
	_ = x[NPY_FLOAT-11]
	_ = x[NPY_DOUBLE-12]
	_ = x[NPY_LONGDOUBLE-13]
	_ = x[NPY_CFLOAT-14]
	_ = x[NPY_CDOUBLE-15]
	_ = x[NPY_CLONGDOUBLE-16]
	_ = x[NPY_OBJECT-17]
	_ = x[NPY_STRING-18]
	_ = x[NPY_UNICODE-19]
	_ = x[NPY_VOID-20]
	_ = x[NPY_DATETIME-21]
	_ = x[NPY_TIMEDELTA-22]
	_ = x[NPY_HALF-23]
}

const _NumpyType_name = "NPY_BOOLNPY_BYTENPY_UBYTENPY_SHORTNPY_USHORTNPY_INTNPY_UINTNPY_LONGNPY_ULONGNPY_LONGLONGNPY_ULONGLONGNPY_FLOATNPY_DOUBLENPY_LONGDOUBLENPY_CFLOATNPY_CDOUBLENPY_CLONGDOUBLENPY_OBJECTNPY_STRINGNPY_UNICODENPY_VOIDNPY_DATETIMENPY_TIMEDELTANPY_HALF"

var _NumpyType_index = [...]uint8{0, 8, 16, 25, 34, 44, 51, 59, 67, 76, 88, 101, 110, 120, 134, 144, 155, 170, 180, 190, 201, 209, 221, 234, 242}

func (i NumpyType) String() string {
	if i < 0 || i >= NumpyType(len(_NumpyType_index)-1) {
		return "NumpyType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _NumpyType_name[_NumpyType_index[i]:_NumpyType_index[i+1]]
}
