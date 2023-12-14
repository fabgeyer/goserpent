package main

import (
	"embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/rs/zerolog/log"
)

//go:embed templates/*
var templateFiles embed.FS

type FunctionSignature struct {
	GoFuncName   string
	Args         []FunctionArgument
	GoReturnType string

	initDone          bool
	CFunctionName     string
	PyMethodDefFlags  string
	ArgsPythonNames   []string
	ArgsCPtrSignature []string
	ArgsGoC           []string
	ArgsCToGo         []string
	ArgsGoNames       []string
}

func (fs *FunctionSignature) init() {
	if fs.initDone {
		return
	}

	fs.CFunctionName = "pyexport_" + ToSnakeCase(fs.GoFuncName)

	fs.ArgsPythonNames = make([]string, len(fs.Args))
	fs.ArgsCPtrSignature = make([]string, len(fs.Args))
	fs.ArgsGoC = make([]string, len(fs.Args))
	fs.ArgsCToGo = make([]string, len(fs.Args))
	fs.ArgsGoNames = make([]string, len(fs.Args))
	for i, arg := range fs.Args {
		fs.ArgsPythonNames[i] = arg.PythonName()
		fs.ArgsCPtrSignature[i] = fmt.Sprintf("%s%s", arg.CPtrType(), arg.PythonName())
		fs.ArgsGoC[i] = fmt.Sprintf("var %s %s", arg.GoName, arg.GoCType())
		fs.ArgsCToGo[i] = fmt.Sprintf("%s(%s)", arg.CToGoFunction(), arg.GoName)
		fs.ArgsGoNames[i] = arg.GoName
	}

	fs.initDone = true
}

func (fs *FunctionSignature) HasArgs() bool {
	return len(fs.Args) != 0
}

func (fs *FunctionSignature) PyArgFormat() string {
	res := ""
	for _, arg := range fs.Args {
		res += arg.PyArgFormat()
	}
	return res
}

func (fs *FunctionSignature) PyModuleDef() string {
	fs.init()
	if fs.HasArgs() {
		return fmt.Sprintf(`{"%s", (PyCFunction)%s, METH_VARARGS | METH_KEYWORDS, ""}`, ToSnakeCase(fs.GoFuncName), fs.CFunctionName)
	} else {
		return fmt.Sprintf(`{"%s", %s, METH_NOARGS, ""}`, ToSnakeCase(fs.GoFuncName), fs.CFunctionName)
	}
}

func (fs *FunctionSignature) GoPyReturn(args []string) string {
	var goCall string
	if len(args) == 0 {
		goCall = fs.GoFuncName + "()"
	} else {
		goCall = fmt.Sprintf("%s(%s)", fs.GoFuncName, strings.Join(args, ", "))
	}

	switch fs.GoReturnType {
	case "": // Equivalent of Python's None
		return fmt.Sprintf("%s; return C.Py_None", goCall)
	case "*C.PyObject":
		return fmt.Sprintf("return %s", goCall)
	case "bool":
		return fmt.Sprintf("return AsPyBool(%s)", goCall)
	case "int":
		return fmt.Sprintf("return AsPyLong(%s)", goCall)
	case "string":
		return fmt.Sprintf("return AsPyString(%s)", goCall)
	case "error":
		return fmt.Sprintf("return AsPyError(%s)", goCall)
	default:
		panic(fmt.Sprintf("Type '%s' not supported for function %s", fs.GoReturnType, fs.GoFuncName))
	}
}

type FunctionArgument struct {
	GoName string
	GoType string
}

func (fa *FunctionArgument) PyArgFormat() string {
	// Returns the format used for PyArg_ParseTupleAndKeywords()
	// https://docs.python.org/3/c-api/arg.html

	switch fa.GoType {
	case "int":
		return "i"
	case "int32":
		return "l"
	case "int64":
		return "L"
	case "uint32":
		return "k"
	case "uint64":
		return "K"
	case "float32":
		return "f"
	case "float64":
		return "d"
	case "string":
		return "s"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", fa.GoType))
	}
}

func (fa *FunctionArgument) PythonName() string {
	return ToSnakeCase(fa.GoName)
}

func (fa *FunctionArgument) GoCType() string {
	switch fa.GoType {
	case "int":
		return "C.int"
	case "int32":
		return "C.int32_t"
	case "int64":
		return "C.int64_t"
	case "uint32":
		return "C.uint32_t"
	case "uint64":
		return "C.uint64_t"
	case "string":
		return "*C.char"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", fa.GoType))
	}
}

func (fa *FunctionArgument) CPtrType() string {
	switch fa.GoType {
	case "int":
		return "int *"
	case "int32":
		return "int32_t *"
	case "int64":
		return "int64_t *"
	case "uint32":
		return "uint32_t *"
	case "uint64":
		return "uint64_t *"
	case "string":
		return "char **"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", fa.GoType))
	}
}

func (fa *FunctionArgument) CToGoFunction() string {
	switch fa.GoType {
	case "int", "uint", "int32", "int64", "uint32", "uint64":
		return fa.GoType
	case "string":
		return "C.GoString"
	default:
		panic(fmt.Sprintf("Type '%s' not supported", fa.GoType))
	}
}

func IsCPyObjectPtr(field *ast.Field) bool {
	sta, ok := field.Type.(*ast.StarExpr)
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

func SafeWriteTemplate(tmpl *template.Template, templateName string, data any, fname string) error {
	var err error
	f, err := os.Create(fname)
	if err != nil {
		return err
	}

	err = tmpl.ExecuteTemplate(f, templateName, data)
	f.Close()
	if err != nil {
		// Cleanup file
		os.Remove(fname)
	}
	return err
}

func GeneratePyExportsCode(cCodeFname, goCodeFname, goPackageName string, fnSignatures []*FunctionSignature, cModuleName string) {
	if len(fnSignatures) == 0 {
		log.Warn().Msg("Did not generate any code!")
		return
	}

	tmpl, err := template.New("gopy").
		Funcs(template.FuncMap{
			"join": strings.Join,
		}).
		ParseFS(templateFiles, "templates/*")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	for _, fs := range fnSignatures {
		fs.init()
	}

	data := struct {
		PackageName string
		CModuleName string
		Functions   []*FunctionSignature
	}{
		PackageName: goPackageName,
		CModuleName: cModuleName,
		Functions:   fnSignatures,
	}

	log.Trace().Str("File", goCodeFname).Msg("Export Go code")
	err = SafeWriteTemplate(tmpl, "maingo", data, goCodeFname)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate Go code")
	}
	err = SafeWriteTemplate(tmpl, "mainc", data, cCodeFname)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate C code")
		// Cleanup generated Go code
		os.Remove(goCodeFname)
	}
}

func FuncDeclDocContains(funcDecl *ast.FuncDecl, substr string) bool {
	if funcDecl.Doc == nil {
		return false
	}

	for _, v := range funcDecl.Doc.List {
		if strings.Contains(v.Text, substr) {
			return true
		}
	}
	return false
}

func DoPyExports(args Args, rargs []string) {
	var fnPackage string
	var fnSignatures []*FunctionSignature

	for _, fname := range rargs {
		log.Trace().Msgf("Process %s", fname)

		content, err := ioutil.ReadFile(fname)
		if err != nil {
			log.Fatal().Str("filename", fname).Err(err).Msg("")
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, fname, content, parser.ParseComments)
		if err != nil {
			log.Fatal().Str("filename", fname).Err(err).Msg("")
		}

		ast.Inspect(f, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.File:
				if fnPackage == "" {
					fnPackage = t.Name.Name
					log.Trace().Msgf("Detected package '%s'", fnPackage)
				} else if t.Name.Name != fnPackage {
					log.Trace().Msgf("Detected package '%s' != '%s'", t.Name.Name, fnPackage)
					log.Fatal().Msg("All files need to be in the same package!")
				}

			// Find all function declarations
			case *ast.FuncDecl:
				isExport := FuncDeclDocContains(t, "go:pyexport")

				// Check if the function returns a *C.PyObject
				numReturnFields := t.Type.Results.NumFields()
				returnsCPythonPtr := (numReturnFields == 1) && IsCPyObjectPtr(t.Type.Results.List[0])

				if !(isExport || returnsCPythonPtr) {
					log.Trace().Msgf("Skip function %s", t.Name.Name)
					return true
				}

				var args []FunctionArgument
				for _, list := range t.Type.Params.List {
					typeIdent, ok := list.Type.(*ast.Ident)
					if !ok {
						log.Fatal().Str("function", t.Name.Name).Msgf("Argument type '%s' not supported!", list.Type)
						return true
					}

					for _, n := range list.Names {
						args = append(args, FunctionArgument{
							GoName: n.Name,
							GoType: typeIdent.Name,
						})
					}
				}

				var goReturnType string
				if returnsCPythonPtr {
					goReturnType = "*C.PyObject"

				} else if numReturnFields == 0 {
					goReturnType = "" // Equivalent of Python's None

				} else if numReturnFields == 1 {
					ident, ok := t.Type.Results.List[0].Type.(*ast.Ident)
					if !ok {
						log.Fatal().Str("function", t.Name.Name).Msgf("Return type '%s' not supported!", t.Type.Results.List[0])
					}
					goReturnType = ident.Name

				} else {
					log.Fatal().Msgf("Invalid return signature for function %s", t.Name.Name)
				}

				fnSignatures = append(fnSignatures, &FunctionSignature{
					GoFuncName:   t.Name.Name,
					Args:         args,
					GoReturnType: goReturnType,
				})
			}
			return true
		})
	}

	GeneratePyExportsCode(args.OutputCCode, args.OutputGoCode, fnPackage, fnSignatures, args.PyModuleName)
}
