# goserpent

`goserpent` generates CPython wrappers from Go functions. The wrapped Go functions can then be directly called from Python.

This project can be seen as a simplified version of [`gopy`](https://github.com/go-python/gopy).
`goserpent` can export Go functions and structures, and the Go code can directly interact with `PyObject` structures.
`gopy` exports Go functions and strutures, and is much more mature than `goserpent`.
Use `goserpent` if you need to export Go functions directly interacting with `PyObject` structures.
Otherwise `gopy` is a better choice.

This project has been successfully tested with Go 1.23 and Python 3.12.

The `textfile.go` shows a few example of exported Go functions.

## Installation

```
$ go install github.com/fabgeyer/goserpent@latest
```

## Usage

To export a Go function to Python, add `go:pyexport` in the function's comment:
```go
// go:pyexport
func ExampleFunction(arg int) int {
	println(arg)
	return arg * arg
}
```
Functions directly returning a `*C.PyObject` are automatically exported and the `go:pyexport` is optional.

Generate the CPython wrappers for the Go functions using:
```
$ goserpent <filename.go>
```

Then compile the CPython module:
```
$ go build -buildmode=c-shared -o gomodule.so
```

The Go functions can be called from Python using it's snakecase name:
```python
from gomodule import example_function
print(example_function(42))
```

`goserpent` also supports functions returning an error.
If the function returns an error, a Python runtime error is thrown when called from Python.
```go
// go:pyexport
func ExampleFunctionWithError(arg int) (int, error) {
	if arg == 0 {
		// Will throw a runtime error when called from Python
		return 0, errors.New("Invalid argument")
	}
	println(arg)
	return arg * arg, nil
}
```

There is experimental support for exporting Go structures to Python. See `testfile.go` for an example.

## Limitations

- `goserpent` only supports a small subset of native Go types for the function's argument and return types.
- `goserpent` does not directly generate and compile a Python package.
