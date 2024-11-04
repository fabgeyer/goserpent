import testmodule as tm

assert tm.FunctionWithArgs(1, 2, "hello") is None

assert tm.BasicFunction() is None

assert tm.BasicFunctionWithError(42) == 42

try:
    tm.BasicFunctionWithError(0)
except RuntimeError:
    pass
else:
    raise Exception("Function did not throw an error")

assert tm.FunctionReturnBool(True) == True

assert tm.FunctionReturnBool(False) == False

assert tm.FunctionReturnNone() is None

assert tm.FunctionReturnInt(42) == 84

assert tm.FunctionReturnUint(42) == 84

assert tm.FunctionReturnIntList(42) == [42, 43]

assert tm.FunctionReturnIntFloat(42) == [42.0, 84.0]

try:
    tm.FunctionReturnError(42)
except RuntimeError:
    pass
else:
    raise Exception("Function did not throw an error")

assert tm.FunctionMapArgument({"a": 1, "b": 2}, "a") == True
assert tm.FunctionMapArgument({"a": 1, "b": 2}, "c") == False

assert tm.FunctionListArgument(list(range(5))) == 5

assert tm.FunctionReturnBytes().decode("utf8") == "Hello world!"

v = tm.NewExportedType(1234)
assert v.GetValue() == 1234
v.Add(1)
assert v.GetValue() == 1235
