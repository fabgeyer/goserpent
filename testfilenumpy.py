import numpy as np
import testmodulenumpy as tmn

x = np.arange(12).reshape(3, 4)
tmn.PrintDescr(x)
tmn.PrintValues(x)

for dtype in [np.float32, np.float64]:
    y = np.random.rand(10, 10).astype(dtype)
    expected = np.copy(y) + 84
    tmn.AddIntValue(y, 84)
    assert np.all(y == expected)

for dtype in [np.int16, np.int32, np.int64]:
    z = np.arange(42).astype(dtype)
    expected = np.copy(z) + 42
    tmn.AddIntValue(z, 42)
    assert np.all(z == expected)
