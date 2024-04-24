package main

type CodegenCommand struct {
}

var codegenCommand CodegenCommand

func (x *CodegenCommand) Execute(rargs []string) error {
	args.Process()
	return DoPyExports(args, rargs)
}

func init() {
	flagparser.AddCommand("codegen",
		"Generate code",
		"The generate command generates the go and C code",
		&codegenCommand)
}
