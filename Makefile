.PHONY: all
all: goserpent

kind_string.go: type.go
	go generate $<
numpy/numpytype_string.go: numpy/array.go
	cd numpy && CGO_CFLAGS="-I$(shell python3 -c 'import numpy; print(numpy.get_include())')" go generate array.go

goserpent: $(shell find templates/*) buildcmd.go codegencmd.go codegencmd_test.go codegen.go kind_string.go main.go testfile.go type.go utils.go numpy/array.go numpy/numpytype_string.go
	go build -o $@

testmodule.so: testfile.go goserpent
	./goserpent build --pymodule=testmodule --tags=python --keep-files $<

testmodulenumpy.so: testfilenumpy.go goserpent
	./goserpent build --pymodule=testmodulenumpy --tags=pythonnumpy --keep-files $<

.PHONY:
test: testmodule.so testmodulenumpy.so
	python3 testfile.py
	python3 testfilenumpy.py
