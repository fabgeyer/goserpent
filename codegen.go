package main

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
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
	GoFuncName       string
	Args             []FunctionArgument
	GoReturnType     GoType
	GoDoc            string
	ReturnsAlsoError bool

	initDone                     bool
	CFunctionName                string
	PyMethodDefFlags             string
	ArgsPythonNames              []string
	ArgsPythonNamesWithTypeHints []string
	ArgsCPtrSignature            []string
	ArgsGoC                      []string
	ArgsCToGo                    []string
	ArgsGoNames                  []string
}

func (fs *FunctionSignature) init() {
	if fs.initDone {
		return
	}
	log.Trace().Msgf("Init %s", fs.GoFuncName)

	fs.CFunctionName = "pyexport_" + ToSnakeCase(fs.GoFuncName)

	fs.ArgsPythonNames = make([]string, len(fs.Args))
	fs.ArgsPythonNamesWithTypeHints = make([]string, len(fs.Args))
	fs.ArgsCPtrSignature = make([]string, len(fs.Args))
	fs.ArgsGoC = make([]string, len(fs.Args))
	fs.ArgsCToGo = make([]string, len(fs.Args))
	fs.ArgsGoNames = make([]string, len(fs.Args))
	for i, arg := range fs.Args {
		fs.ArgsPythonNames[i] = arg.PythonName()
		fs.ArgsPythonNamesWithTypeHints[i] = fmt.Sprintf("%s: %s", arg.PythonName(), arg.PythonType())
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
	pyFunctionName := ToSnakeCase(fs.GoFuncName)
	doc, err := fs.PyModuleDefDoc(pyFunctionName)
	if err != nil {
		log.Fatal().
			Str("function", fs.GoFuncName).
			Err(err).
			Msg("Could not generate documentation")
	}

	if fs.HasArgs() {
		return fmt.Sprintf(`{"%s", (PyCFunction)%s, METH_VARARGS | METH_KEYWORDS, %s}`, pyFunctionName, fs.CFunctionName, doc)
	} else {
		return fmt.Sprintf(`{"%s", %s, METH_NOARGS, %s}`, pyFunctionName, fs.CFunctionName, doc)
	}
}

func CCodeString(v string) (string, error) {
	if strings.ContainsRune(v, '\n') {
		if strings.Contains(v, `")`) {
			// TODO: Properly escape quotes
			return "", errors.New("String contains quote")
		}
		return fmt.Sprintf(`R"(%s)"`, v), nil
	}

	if strings.ContainsRune(v, '"') {
		// TODO: Properly escape quotes
		return "", errors.New("String contains quote")
	}
	return fmt.Sprintf(`"%s"`, v), nil
}

func (fs *FunctionSignature) PyModuleDefDoc(pyFunctionName string) (string, error) {
	fs.init()

	var returnSignature string
	if fs.GoReturnType != "" && fs.GoReturnType != "error" {
		returnSignature = fmt.Sprintf(" -> %s", fs.GoReturnType.PythonType())
	}

	signature := fmt.Sprintf("%s(%s)%s", pyFunctionName, strings.Join(fs.ArgsPythonNamesWithTypeHints, ", "), returnSignature)
	if fs.GoDoc == "" {
		return CCodeString(signature)
	} else {
		return CCodeString(signature + "\n\n" + fs.GoDoc)
	}
}

func (fs *FunctionSignature) GoPyReturn(result string) string {
	switch fs.GoReturnType {
	case "": // Equivalent of Python's None
		return "return C.Py_None"
	case "*C.PyObject":
		return fmt.Sprintf("return %s", result)
	case "bool":
		return fmt.Sprintf("return asPyBool(%s)", result)
	case "int", "int8", "int16", "int32":
		return fmt.Sprintf("return asPyLong(%s)", result)
	case "float32", "float64":
		return fmt.Sprintf("return asPyFloat(%s)", result)
	case "string":
		return fmt.Sprintf("return asPyString(%s)", result)
	case "error":
		return fmt.Sprintf("return asPyError(%s)", result)
	default:
		panic(fmt.Sprintf("Type '%s' not supported for function %s", fs.GoReturnType, fs.GoFuncName))
	}
}

type FunctionArgument struct {
	GoType
	GoName string
}

func (fa *FunctionArgument) PythonName() string {
	return ToSnakeCase(fa.GoName)
}

func (fa *FunctionArgument) CToGoFunction() string {
	switch fa.GoType {
	case "*C.PyObject":
		return ""
	case "bool":
		return "asGoBool"
	case "int", "uint", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "float32", "float64":
		return string(fa.GoType)
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

func IsErrorType(field *ast.Field) bool {
	ident, ok := field.Type.(*ast.Ident)
	if !ok {
		return false
	}
	if ident.Name == "error" {
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

	log.Trace().Str("filename", goCodeFname).Msg("Export Go code")
	err = SafeWriteTemplate(tmpl, "maingo", data, goCodeFname)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate Go code")
	}

	log.Trace().Str("filename", cCodeFname).Msg("Export C code")
	err = SafeWriteTemplate(tmpl, "mainc", data, cCodeFname)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate C code")
		// Cleanup generated Go code
		os.Remove(goCodeFname)
	}
}

func ProcessFunc(fn *doc.Func, sourceContent []byte) *FunctionSignature {
	var fnDoc string
	var isExport bool

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(fn.Doc)))
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.HasPrefix(txt, "go:pyexport") {
			isExport = true
		} else {
			if fnDoc == "" {
				fnDoc = txt
			} else {
				fnDoc += "\n" + txt
			}
		}
	}

	numReturnFields := fn.Decl.Type.Results.NumFields()
	// Check if the function returns a *C.PyObject
	returnsCPythonPtr := (numReturnFields == 1) && IsCPyObjectPtr(fn.Decl.Type.Results.List[0])

	if !(isExport || returnsCPythonPtr) {
		log.Trace().Msgf("Skip function %s", fn.Name)
		return nil
	}

	var args []FunctionArgument
	for _, list := range fn.Decl.Type.Params.List {
		typeIdent, ok := list.Type.(*ast.Ident)
		if ok {
			for _, n := range list.Names {
				args = append(args, FunctionArgument{
					GoName: n.Name,
					GoType: GoType(typeIdent.Name),
				})
			}
			continue
		}

		if IsCPyObjectPtr(list) {
			for _, n := range list.Names {
				args = append(args, FunctionArgument{
					GoName: n.Name,
					GoType: "*C.PyObject",
				})
			}
			continue
		}

		log.Fatal().
			Str("function", fn.Name).
			Msgf("Argument type '%s' not supported!", GetSourceString(sourceContent, list.Type))
		return nil
	}

	var goReturnType string
	var returnsAlsoError bool
	if returnsCPythonPtr {
		goReturnType = "*C.PyObject"

	} else if numReturnFields == 0 {
		goReturnType = "" // Equivalent of Python's None

	} else if numReturnFields == 1 || numReturnFields == 2 {
		ident, ok := fn.Decl.Type.Results.List[0].Type.(*ast.Ident)
		if !ok {
			log.Fatal().
				Str("function", fn.Name).
				Msgf("Return type '%s' not supported!", GetSourceString(sourceContent, fn.Decl.Type.Results.List[0]))
		}
		goReturnType = ident.Name

		if numReturnFields == 2 {
			if IsErrorType(fn.Decl.Type.Results.List[1]) {
				returnsAlsoError = true
			} else {
				log.Fatal().
					Str("function", fn.Name).
					Msgf("Return type '%s' not supported!", GetSourceString(sourceContent, fn.Decl.Type.Results.List[1]))
			}
		}

	} else {
		log.Fatal().Msgf("Invalid return signature for function %s", fn.Name)
	}

	return &FunctionSignature{
		GoFuncName:       fn.Name,
		Args:             args,
		GoReturnType:     GoType(goReturnType),
		GoDoc:            strings.TrimSpace(fnDoc),
		ReturnsAlsoError: returnsAlsoError,
	}
}

func GetSourceString(content []byte, node ast.Node) string {
	return string(content[node.Pos()-1 : node.End()-1])
}

func DoPyExports(args Args, fnames []string) {
	var fnPackage string
	var fnSignatures []*FunctionSignature

	for _, fname := range fnames {
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

		pkg, err := doc.NewFromFiles(fset, []*ast.File{f}, "", doc.PreserveAST)
		if err != nil {
			log.Fatal().Str("filename", fname).Err(err).Msg("")
		}

		if fnPackage == "" {
			fnPackage = pkg.Name
			log.Trace().Msgf("Detected Go package '%s'", fnPackage)
		} else if pkg.Name != fnPackage {
			log.Trace().Msgf("Detected Go package '%s' != '%s'", pkg.Name, fnPackage)
			log.Fatal().Msg("All files need to be in the same package!")
		}

		for _, fn := range pkg.Funcs {
			if fn.Level != 0 {
				continue
			}
			fs := ProcessFunc(fn, content)
			if fs != nil {
				log.Debug().
					Str("filename", fname).
					Msgf("Exporting %s", fn.Name)
				fnSignatures = append(fnSignatures, fs)
			}
		}
	}

	GeneratePyExportsCode(args.OutputCCode, args.OutputGoCode, fnPackage, fnSignatures, args.PyModuleName)
}
