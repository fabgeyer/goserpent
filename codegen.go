package main

import (
	"bufio"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
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
	GoRecv           string
	CRecv            string
	CGoRecv          string
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

	nargs := len(fs.Args)
	if fs.GoRecv == "" {
		fs.CFunctionName = "pyexport_" + ToSnakeCase(fs.GoFuncName)
	} else {
		fs.CFunctionName = fmt.Sprintf("pyexport_%s_%s", ToSnakeCase(fs.GoRecv[1:]), ToSnakeCase(fs.GoFuncName))
		fs.CRecv = fs.GoRecv[1:]
		fs.CGoRecv = "*C." + fs.CRecv
	}

	fs.ArgsPythonNames = make([]string, nargs)
	fs.ArgsPythonNamesWithTypeHints = make([]string, nargs)
	fs.ArgsCPtrSignature = make([]string, nargs)
	fs.ArgsGoC = make([]string, nargs)
	fs.ArgsCToGo = make([]string, nargs)
	fs.ArgsGoNames = make([]string, nargs)

	i := 0
	for _, arg := range fs.Args {
		fs.ArgsPythonNames[i] = arg.PythonName()
		fs.ArgsPythonNamesWithTypeHints[i] = fmt.Sprintf("%s: %s", arg.PythonName(), arg.PythonType())
		fs.ArgsCPtrSignature[i] = fmt.Sprintf("%s%s", arg.CPtrType(), arg.PythonName())
		fs.ArgsGoC[i] = fmt.Sprintf("var %s %s", arg.GoName, arg.GoCType())
		fs.ArgsCToGo[i] = fmt.Sprintf("%s(%s)", arg.CToGoFunction(), arg.GoName)
		fs.ArgsGoNames[i] = arg.GoName
		i++
	}

	fs.initDone = true
}

func (fs *FunctionSignature) HasArgs() bool {
	return len(fs.Args) != 0
}

func (fs *FunctionSignature) HasRecv() bool {
	return fs.GoRecv != ""
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
			Caller().
			Str("function", fs.GoFuncName).
			Err(err).
			Msg("Could not generate documentation")
	}

	if fs.HasArgs() || fs.HasRecv() {
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
		if strings.HasPrefix(string(fs.GoReturnType), "*") {
			return fmt.Sprintf("return C.new_%s(C.uintptr_t(cgo.NewHandle(%s)))", fs.GoReturnType[1:], result)
		}
		log.Fatal().Caller().Msgf("Type '%s' not supported for function %s", fs.GoReturnType, fs.GoFuncName)
		panic("")
	}
}

type CythonDictToGoMap struct {
	GoMapKeyType GoType
	GoMapValType GoType
}

var requiredCythonDictToGoMap map[CythonDictToGoMap]bool = make(map[CythonDictToGoMap]bool)

type FunctionArgument struct {
	GoType
	GoMapKeyType GoType
	GoMapValType GoType
	GoName       string
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
	case "map":
		requiredCythonDictToGoMap[CythonDictToGoMap{
			GoMapKeyType: fa.GoMapKeyType,
			GoMapValType: fa.GoMapValType,
		}] = true
		return fmt.Sprintf("CythonDictToGoMap_%s_%s", fa.GoMapKeyType, fa.GoMapValType)
	default:
		log.Fatal().Caller().Msgf("Type '%s' not supported", fa.GoType)
		panic("")
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

func GeneratePyExportsCode(cCodeFname, cHeaderFname, goCodeFname, goPackageName string, goTags []string, fnSignatures []*FunctionSignature, tpSignatures []*TypeSignature, cModuleName string) error {
	if len(fnSignatures) == 0 {
		return errors.New("No function signature exported")
	}

	tmpl, err := template.New("gopy").
		Funcs(template.FuncMap{
			"join":         strings.Join,
			"pyObjectToGo": TplCPyObjectToGo,
		}).
		ParseFS(templateFiles, "templates/*")
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("")
	}

	for _, fs := range fnSignatures {
		fs.init()
	}

	requiresRuntimeCgo := false
	for _, ts := range tpSignatures {
		ts.init()
		requiresRuntimeCgo = requiresRuntimeCgo || len(ts.Methods) > 0 || len(ts.Funcs) > 0
	}

	requiredCythonDictToGoMapList := make([]CythonDictToGoMap, len(requiredCythonDictToGoMap))
	i := 0
	for k, _ := range requiredCythonDictToGoMap {
		requiredCythonDictToGoMapList[i] = k
		i++
	}

	var imports []string
	if requiresRuntimeCgo {
		imports = append(imports, "runtime/cgo")
	}

	data := struct {
		GoTags                    string
		PackageName               string
		CModuleName               string
		CHeaderFname              string
		Functions                 []*FunctionSignature
		Types                     []*TypeSignature
		RequiredCythonDictToGoMap []CythonDictToGoMap
		Imports                   []string
	}{
		GoTags:                    strings.Join(goTags, " "),
		PackageName:               goPackageName,
		CModuleName:               cModuleName,
		CHeaderFname:              cHeaderFname,
		Functions:                 fnSignatures,
		Types:                     tpSignatures,
		RequiredCythonDictToGoMap: requiredCythonDictToGoMapList,
		Imports:                   imports,
	}

	log.Trace().Str("filename", goCodeFname).Msg("Export Go code")
	var gosrcbuf bytes.Buffer
	err = tmpl.ExecuteTemplate(&gosrcbuf, "maingo", data)
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("Failed to generate Go code")
	}
	gosrc, err := format.Source(gosrcbuf.Bytes())
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("Failed to format Go code")
	}
	err = os.WriteFile(goCodeFname, gosrc, 0644)
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("Failed to write file")
	}

	log.Trace().Str("filename", cCodeFname).Msg("Export C code")
	err = SafeWriteTemplate(tmpl, "mainc", data, cCodeFname)
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("Failed to generate C code")
		// Cleanup generated Go code
		os.Remove(goCodeFname)
	}

	log.Trace().Str("filename", cHeaderFname).Msg("Export C header")
	err = SafeWriteTemplate(tmpl, "mainchdr", data, cHeaderFname)
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("Failed to generate C header")
		// Cleanup generated Go code
		os.Remove(goCodeFname)
		os.Remove(cCodeFname)
		return err
	}
	return nil
}

func ProcessDoc(doc string) (string, bool) {
	var fnDoc string
	var isExport bool

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(doc)))
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
	return fnDoc, isExport
}

func ProcessFunc(fn *doc.Func, sourceContent []byte) *FunctionSignature {
	fnDoc, isExport := ProcessDoc(fn.Doc)

	numReturnFields := fn.Decl.Type.Results.NumFields()
	// Check if the function returns a *C.PyObject
	returnsCPythonPtr := (numReturnFields == 1) && IsCPyObjectPtr(fn.Decl.Type.Results.List[0])

	if !(isExport || returnsCPythonPtr) {
		log.Trace().Msgf("Skip function %s", fn.Name)
		return nil
	}

	var args []FunctionArgument
	for _, list := range fn.Decl.Type.Params.List {
		switch t := list.Type.(type) {
		case *ast.Ident:
			for _, n := range list.Names {
				args = append(args, FunctionArgument{
					GoName: n.Name,
					GoType: GoType(t.Name),
				})
			}
			continue

		case *ast.MapType:
			for _, n := range list.Names {
				args = append(args, FunctionArgument{
					GoName:       n.Name,
					GoType:       "map",
					GoMapKeyType: GoType(t.Key.(*ast.Ident).Name),
					GoMapValType: GoType(t.Value.(*ast.Ident).Name),
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
			Caller().
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
		switch v := fn.Decl.Type.Results.List[0].Type.(type) {
		case *ast.Ident:
			goReturnType = v.Name

		case *ast.StarExpr:
			// Pointer value
			ident, ok := v.X.(*ast.Ident)
			if !ok {
				log.Fatal().
					Caller().
					Str("function", fn.Name).
					Str("type", fmt.Sprintf("%T", v.X)).
					Msgf("Return type '%s' not supported!", GetSourceString(sourceContent, v.X))
			}
			goReturnType = "*" + ident.Name

		default:
			log.Fatal().
				Caller().
				Str("function", fn.Name).
				Str("type", fmt.Sprintf("%T", v)).
				Msgf("Return type '%s' not supported!", GetSourceString(sourceContent, v))
		}

		if numReturnFields == 2 {
			if IsErrorType(fn.Decl.Type.Results.List[1]) {
				returnsAlsoError = true
			} else {
				log.Fatal().
					Caller().
					Str("function", fn.Name).
					Str("type", fmt.Sprintf("%T", fn.Decl.Type.Results.List[1])).
					Msgf("Return type '%s' not supported!", GetSourceString(sourceContent, fn.Decl.Type.Results.List[1]))
			}
		}

	} else {
		log.Fatal().
			Caller().
			Msgf("Invalid return signature for function %s", fn.Name)
	}

	var recv string
	if fn.Recv != "" {
		if !strings.HasPrefix(fn.Recv, "*") {
			log.Fatal().
				Caller().
				Msgf("Non-pointer receiver not supported for %s", fn.Recv)
		} else {
			recv = fn.Recv
		}
	}

	return &FunctionSignature{
		GoFuncName:       fn.Name,
		Args:             args,
		GoReturnType:     GoType(goReturnType),
		GoDoc:            strings.TrimSpace(fnDoc),
		GoRecv:           recv,
		ReturnsAlsoError: returnsAlsoError,
	}
}

type TypeSignature struct {
	GoTypeName       string
	PyTypeObjectName string
	GoDoc            string
	Methods          []*FunctionSignature
	Funcs            []*FunctionSignature
}

func (ts *TypeSignature) init() {
	for _, m := range ts.Methods {
		m.init()
	}
	for _, f := range ts.Funcs {
		f.init()
	}
}

func ProcessType(tp *doc.Type, sourceContent []byte) *TypeSignature {
	var methods []*FunctionSignature
	for _, fn := range tp.Methods {
		if fn.Level != 0 {
			continue
		}
		fs := ProcessFunc(fn, sourceContent)
		if fs != nil {
			methods = append(methods, fs)
		}
	}

	var funcs []*FunctionSignature
	for _, fn := range tp.Funcs {
		if fn.Level != 0 {
			continue
		}
		fs := ProcessFunc(fn, sourceContent)
		if fs != nil {
			funcs = append(funcs, fs)
		}
	}

	if len(methods) == 0 && len(funcs) == 0 {
		return nil
	}

	tpDoc, _ := ProcessDoc(tp.Doc)

	return &TypeSignature{
		GoTypeName:       tp.Name,
		PyTypeObjectName: fmt.Sprintf("PyTo_%s", tp.Name),
		GoDoc:            tpDoc,
		Methods:          methods,
		Funcs:            funcs,
	}
}

func GetSourceString(content []byte, node ast.Node) string {
	return string(content[node.Pos()-1 : node.End()-1])
}

func DoPyExports(args Args, fnames []string) error {
	var fnPackage string
	var fnSignatures []*FunctionSignature
	var tpSignatures []*TypeSignature

	for _, fname := range fnames {
		log.Trace().Msgf("Process %s", fname)

		content, err := ioutil.ReadFile(fname)
		if err != nil {
			log.Fatal().Caller().Str("filename", fname).Err(err).Msg("")
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, fname, content, parser.ParseComments)
		if err != nil {
			log.Fatal().Caller().Str("filename", fname).Err(err).Msg("")
		}

		pkg, err := doc.NewFromFiles(fset, []*ast.File{f}, "", doc.PreserveAST)
		if err != nil {
			log.Fatal().Caller().Str("filename", fname).Err(err).Msg("")
		}

		if fnPackage == "" {
			fnPackage = pkg.Name
			log.Trace().Msgf("Detected Go package '%s'", fnPackage)
		} else if pkg.Name != fnPackage {
			log.Trace().Msgf("Detected Go package '%s' != '%s'", pkg.Name, fnPackage)
			log.Fatal().Caller().Msg("All files need to be in the same package!")
		}

		for _, fn := range pkg.Funcs {
			if fn.Level != 0 {
				continue
			}
			fs := ProcessFunc(fn, content)
			if fs == nil {
				continue
			}

			log.Debug().
				Str("filename", fname).
				Msgf("Exporting %s", fn.Name)
			fnSignatures = append(fnSignatures, fs)
		}

		for _, tp := range pkg.Types {
			ts := ProcessType(tp, content)
			if ts == nil {
				continue
			}

			for _, m := range ts.Methods {
				log.Debug().
					Str("filename", fname).
					Msgf("Exporting %s.%s", tp.Name, m.GoFuncName)
			}
			tpSignatures = append(tpSignatures, ts)
		}
	}

	return GeneratePyExportsCode(args.OutputCCode, args.OutputChdrCode, args.OutputGoCode, fnPackage, args.GoTags, fnSignatures, tpSignatures, args.PyModuleName)
}
