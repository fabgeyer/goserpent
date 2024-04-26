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
	GoReturnType     *GoType
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
		if args.UseSnakeCase {
			fs.CFunctionName = "pyexport_" + fs.GoFuncName
		} else {
			fs.CFunctionName = "pyexport_" + ToSnakeCase(fs.GoFuncName)
		}

	} else {
		if args.UseSnakeCase {
			fs.CFunctionName = fmt.Sprintf("pyexport_%s_%s", ToSnakeCase(fs.GoRecv[1:]), ToSnakeCase(fs.GoFuncName))
		} else {
			fs.CFunctionName = fmt.Sprintf("pyexport_%s_%s", fs.GoRecv[1:], fs.GoFuncName)
		}
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
		fs.ArgsPythonNamesWithTypeHints[i] = fmt.Sprintf("%s: %s", arg.PythonName(), arg.PythonTypeHint())
		fs.ArgsCPtrSignature[i] = fmt.Sprintf("%s%s", arg.CPtrType(), arg.PythonName())
		fs.ArgsGoC[i] = fmt.Sprintf("var %s %s", arg.GoName, arg.GoCType())
		fs.ArgsCToGo[i] = arg.CToGoFunction(arg.GoName)
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
	var pyFunctionName string
	if args.UseSnakeCase {
		pyFunctionName = ToSnakeCase(fs.GoFuncName)
	} else {
		pyFunctionName = fs.GoFuncName
	}

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
	if fs.GoReturnType.T != None && fs.GoReturnType.T != Error {
		returnSignature = fmt.Sprintf(" -> %s", fs.GoReturnType.PythonTypeHint())
	}

	signature := fmt.Sprintf("%s(%s)%s", pyFunctionName, strings.Join(fs.ArgsPythonNamesWithTypeHints, ", "), returnSignature)
	if fs.GoDoc == "" {
		return CCodeString(signature)
	} else {
		return CCodeString(signature + "\n\n" + fs.GoDoc)
	}
}

func (fs *FunctionSignature) GoPyReturn(result string) string {
	return fs.GoReturnType.GoPyReturn(result)
}

type FunctionArgument struct {
	*GoType
	GoName string
}

func (fa *FunctionArgument) PythonName() string {
	if args.UseSnakeCase {
		return ToSnakeCase(fa.GoName)
	} else {
		return fa.GoName
	}
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

func SafeWriteTemplate(tmpl *template.Template, templateName string, data any, fname string, formatter func([]byte) ([]byte, error)) error {
	var err error
	if formatter == nil {
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

	} else {
		var buf bytes.Buffer
		err = tmpl.ExecuteTemplate(&buf, templateName, data)
		if err != nil {
			return err
		}

		text, err := formatter(buf.Bytes())
		if err != nil {
			return err
		}

		return os.WriteFile(fname, text, 0644)
	}
}

func GeneratePyExportsCode(cCodeFname, cHeaderFname, goCodeFname, goPackageName string, goTags []string, fnSignatures []*FunctionSignature, tpSignatures []*TypeSignature, cModuleName string) error {
	if len(fnSignatures) == 0 {
		return errors.New("No function signature exported")
	}

	tmpl, err := template.New("gopy").
		Funcs(template.FuncMap{
			"join": strings.Join,
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

	var imports []string
	if requiresRuntimeCgo {
		imports = append(imports, "runtime/cgo")
	}

	data := struct {
		GoTags       string
		PackageName  string
		CModuleName  string
		CHeaderFname string
		Functions    []*FunctionSignature
		Types        []*TypeSignature
		Imports      []string
	}{
		GoTags:       strings.Join(goTags, " "),
		PackageName:  goPackageName,
		CModuleName:  cModuleName,
		CHeaderFname: cHeaderFname,
		Functions:    fnSignatures,
		Types:        tpSignatures,
		Imports:      imports,
	}

	cleanupFiles := func() {
		for _, fname := range []string{goCodeFname, cCodeFname, cHeaderFname} {
			os.Remove(fname)
		}
	}

	withGoFormatting := true
	log.Trace().Str("filename", goCodeFname).Msg("Export Go code")
	if withGoFormatting {
		err = SafeWriteTemplate(tmpl, "maingo", data, goCodeFname, format.Source)
	} else {
		err = SafeWriteTemplate(tmpl, "maingo", data, goCodeFname, RemoveEmptyLines)
	}
	if err != nil {
		cleanupFiles()
		log.Fatal().Caller().Err(err).Msg("Failed to generate Go code")
		return err
	}

	log.Trace().Str("filename", cCodeFname).Msg("Export C code")
	err = SafeWriteTemplate(tmpl, "mainc", data, cCodeFname, RemoveEmptyLines)
	if err != nil {
		cleanupFiles()
		log.Fatal().Caller().Err(err).Msg("Failed to generate C code")
		return err
	}

	log.Trace().Str("filename", cHeaderFname).Msg("Export C header")
	err = SafeWriteTemplate(tmpl, "mainchdr", data, cHeaderFname, RemoveEmptyLines)
	if err != nil {
		cleanupFiles()
		log.Fatal().Caller().Err(err).Msg("Failed to generate C header")
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
	returnsCPythonPtr := (numReturnFields == 1) && IsCPyObjectPtr(fn.Decl.Type.Results.List[0].Type)

	if !args.ExportAll && !(isExport || returnsCPythonPtr) {
		log.Trace().Msgf("Skip function %s", fn.Name)
		return nil
	}

	var args []FunctionArgument
	for _, list := range fn.Decl.Type.Params.List {
		goType, err := AsGoType(list.Type, sourceContent)
		if err != nil {
			log.Fatal().
				Caller().
				Str("function", fn.Name).
				Msgf("Argument type '%s' not supported!", GetSourceString(sourceContent, list.Type))
		}

		for _, n := range list.Names {
			if err != nil {
				log.Fatal().Caller().Err(err).Send()
			}
			args = append(args, FunctionArgument{
				GoName: n.Name,
				GoType: goType,
			})
		}
	}

	var goReturnType *GoType
	var returnsAlsoError bool
	if returnsCPythonPtr {
		goReturnType = &GoType{T: CPyObjectPointer}

	} else if numReturnFields == 0 {
		goReturnType = &GoType{T: None} // Equivalent of Python's None

	} else if numReturnFields == 1 || numReturnFields == 2 {
		var err error
		goReturnType, err = AsGoType(fn.Decl.Type.Results.List[0].Type, sourceContent)
		if err != nil {
			log.Fatal().
				Caller().
				Err(err).
				Str("function", fn.Name).
				Send()
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
		GoReturnType:     goReturnType,
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
