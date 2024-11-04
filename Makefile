.PHONY: all
all: goserpent

kind_string.go: type.go
	go generate

goserpent: $(shell find templates/*) buildcmd.go codegencmd.go codegencmd_test.go codegen.go kind_string.go main.go testfile.go type.go utils.go
	go build -o $@

testmodule.so: testfile.go goserpent
	./goserpent build --pymodule=testmodule --tags=python --keep-files $<

.PHONY:
test: testmodule.so
	python3 testfile.py
