package main

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestCodegenCmd(t *testing.T) {
	args := Args{
		OutputDir:      "",
		OutputCCode:    "pyexports.c",
		OutputChdrCode: "pyexports.h",
		OutputGoCode:   "pyexports.go",
		PyModuleName:   "gomodule",
		GoTags:         []string{"python"},
	}

	DoPyExports(args, []string{"testfile.go"})

	cmd := exec.Command("go", "build", "-buildmode=c-shared",
		fmt.Sprintf("--tags=%s", args.GoTags[0]), "-o", "testmodule.so")
	t.Log("Building with:", cmd)
	cmdout, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Compilation error: %v. Output: %s", err, cmdout)
	}

	cmdout, err = exec.Command("python3", "testfile.py").CombinedOutput()
	if err != nil {
		t.Fatalf("Compilation error: %v. Output: %s", err, cmdout)
	}
}
