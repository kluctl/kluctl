package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"reflect"
	"strings"
)

var inputFile = flag.String("input", "", "input .go file")
var outputFile = flag.String("output", "", "output .go file")

func main() {
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		log.Fatalf("Missing parameters")
	}

	src, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		log.Fatal(err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, *inputFile, src, 0)
	if err != nil {
		log.Fatal(err)
	}

	v := &collector{}
	ast.Walk(v, f)

	goCode := ""
	cCode := ""
	for _, g := range v.gens {
		gc, cc := g.generateImpl()

		goCode += gc
		cCode += cc
	}

	goFile := "package python\n"
	goFile += "/*\n"
	goFile += "#include <assert.h>\n"
	goFile += "#include <stdlib.h>\n"
	goFile += "#include <sys/types.h>\n"
	goFile += cCode
	goFile += "*/\n"
	goFile += "import \"C\"\n"
	goFile += "\n"
	goFile += "import \"unsafe\"\n"
	goFile += "import \"github.com/codablock/kluctl/pkg/utils/lib_wrapper\"\n"
	goFile += goCode

	formatted, err := format.Source([]byte(goFile))
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(*outputFile, []byte(formatted), 0o600)
	if err != nil {
		log.Fatal(err)
	}
}

type collector struct {
	gens []*generator
}

func (v *collector) Visit(n ast.Node) ast.Visitor {
	ce, ok := n.(*ast.TypeSpec)
	if !ok {
		return v
	}
	it, ok := ce.Type.(*ast.InterfaceType)
	if !ok {
		return v
	}

	v.gens = append(v.gens, &generator{
		typ:   ce,
		iface: it,
	})

	return v
}

type generator struct {
	typ   *ast.TypeSpec
	iface *ast.InterfaceType
}

func (g *generator) generateImpl() (string, string) {

	structDecls := "module *lib_wrapper.LibWrapper\n"
	initCalls := ""
	goFuncs := ""
	cFuncs := ""

	for _, m := range g.iface.Methods.List {
		ft, _ := m.Type.(*ast.FuncType)

		structDecls += fmt.Sprintf("_func_%s lib_wrapper.FunctionPtr\n", m.Names[0].String())
		initCalls += fmt.Sprintf("w._func_%s = w.module.GetFunc(\"%s\")\n", m.Names[0].String(), m.Names[0].String())
		goFuncs += g.generateGoFunc(m.Names[0].String(), ft)
		cFuncs += g.generateCFunc(m.Names[0].String(), ft)
	}

	var goCode string

	goCode += fmt.Sprintf("type %sImpl struct {\n", g.typ.Name.String())
	goCode += indent(structDecls, 1)
	goCode += "}\n"

	goCode += fmt.Sprintf("func New_%s(module *lib_wrapper.LibWrapper) %s {\n", g.typ.Name.String(), g.typ.Name.String())
	goCode += fmt.Sprintf("\tw := &%sImpl{module: module}\n", g.typ.Name.String())
	goCode += indent(initCalls, 1)
	goCode += "\treturn w\n"
	goCode += "}\n"

	goCode += goFuncs

	return goCode, cFuncs
}

func exprToString(e ast.Expr) string {
	if x, ok := e.(*ast.Ident); ok {
		return x.String()
	} else if x, ok := e.(*ast.SelectorExpr); ok {
		return fmt.Sprintf("%s.%s", exprToString(x.X), exprToString(x.Sel))
	} else if x, ok := e.(*ast.ArrayType); ok {
		return fmt.Sprintf("[]%s", exprToString(x.Elt))
	} else if x, ok := e.(*ast.StarExpr); ok {
		return fmt.Sprintf("*%s", x.X)
	}
	log.Fatalf("unknown type %s", reflect.TypeOf(e).String())
	return ""
}

func fieldListToString(fl *ast.FieldList) string {
	var args []string
	for _, p := range fl.List {
		var names []string
		for _, n := range p.Names {
			names = append(names, n.String())
		}
		var a string
		if len(names) == 0 {
			a = exprToString(p.Type)
		} else {
			a = fmt.Sprintf("%s %s", strings.Join(names, ", "), exprToString(p.Type))
		}
		args = append(args, a)
	}
	return strings.Join(args, ", ")
}

func (g *generator) generateGoFunc(name string, m *ast.FuncType) string {
	implName := g.typ.Name.String() + "Impl"

	funcStr := fmt.Sprintf("func (w* %s) %s(%s)", implName, name, fieldListToString(m.Params))

	if m.Results != nil && len(m.Results.List) != 0 {
		if len(m.Results.List) != 1 || len(m.Results.List[0].Names) != 0 {
			log.Fatalf("%s: only one result allowed", name)
		}
		funcStr += fmt.Sprintf(" (%s)", fieldListToString(m.Results))
	}

	funcStr += " {\n"
	funcStr += indent(g.generateTrampolineCall(name, m), 1)
	funcStr += "}\n"

	return funcStr
}

func (g *generator) generateTrampolineCall(name string, m *ast.FuncType) string {
	result := ""

	funcArgs := []string{fmt.Sprintf("unsafe.Pointer(w._func_%s)", name)}
	for _, p := range m.Params.List {
		for _, n := range p.Names {
			t := exprToString(p.Type)
			a, stmt := g.generateToCConversion(n.String(), n.String(), t)
			result += stmt
			funcArgs = append(funcArgs, a...)
		}
	}
	call := fmt.Sprintf("C._trampoline_%s(%s)", name, strings.Join(funcArgs, ", "))

	if m.Results != nil && len(m.Results.List) != 0 {
		t := exprToString(m.Results.List[0].Type)
		a, stmt := g.generateToGoConversion("ret", call, t)
		result += stmt
		result += fmt.Sprintf("return %s\n", a)
	} else {
		result += fmt.Sprintf("%s\n", call)
	}
	return result
}

func (g *generator) generateToCConversion(name string, a string, t string) ([]string, string) {
	cn := fmt.Sprintf("_c_%s", name)
	switch t {
	case "int", "ssize_t":
		stmt := fmt.Sprintf("%s := C.%s(%s)\n", cn, t, a)
		return []string{cn}, stmt
	case "string":
		stmt := fmt.Sprintf("%s := C.CString(%s)\n", cn, a)
		stmt += fmt.Sprintf("defer C.free(unsafe.Pointer(%s))\n", cn)
		return []string{cn}, stmt
	case "unsafe.Pointer":
		return []string{a}, ""
	}
	if strings.HasPrefix(t, "[]") {
		t = t[2:]
		stmt := fmt.Sprintf("%s_ := %s\n", cn, a)
		if t == "unsafe.Pointer" {
			stmt += fmt.Sprintf("var %s *unsafe.Pointer\n", cn)
			stmt += fmt.Sprintf("if len(%s_) != 0 { %s = &%s_[0] }\n", cn, cn, cn)
		} else {
			stmt += fmt.Sprintf("var %s *unsafe.Pointer\n", cn)
			stmt += fmt.Sprintf("%s__ := make([]unsafe.Pointer, len(%s_), len(%s_))\n", cn, cn, cn)
			stmt += fmt.Sprintf("for i, _ := range %s_ { %s__[i] = %s_[i].GetPointer() }\n", cn, cn, cn)
			stmt += fmt.Sprintf("if len(%s_) != 0 { %s = &%s__[0] }\n", cn, cn, cn)
		}
		if strings.HasSuffix(name, "_vargs") {
			stmt += fmt.Sprintf("%s_len := len(%s_)\n", cn, cn)
			return []string{fmt.Sprintf("C.int(%s_len)", cn), cn}, stmt
		} else {
			return []string{cn}, stmt
		}
	}
	if strings.HasPrefix(t, "*") {
		stmt := fmt.Sprintf("%s := (%s).GetPointer()\n", cn, a)
		return []string{cn}, stmt
	}
	log.Fatalf("unknown type %s", t)
	return nil, ""
}

func (g *generator) generateToGoConversion(name string, a string, t string) (string, string) {
	gn := fmt.Sprintf("_g_%s", name)
	switch t {
	case "int", "ssize_t":
		stmt := fmt.Sprintf("%s := int(%s)\n", gn, a)
		return gn, stmt
	case "string":
		stmt := fmt.Sprintf("%s := C.GoString(%s)\n", gn, a)
		return gn, stmt
	case "unsafe.Pointer":
		stmt := fmt.Sprintf("%s := unsafe.Pointer(%s)\n", gn, a)
		return gn, stmt
	}
	if strings.HasPrefix(t, "*") {
		stmt := fmt.Sprintf("%s := %s_FromPointer(%s)\n", gn, t[1:], a)
		return gn, stmt
	}
	log.Fatalf("unknown type %s", t)
	return "", ""
}

func (g *generator) generateCFunc(name string, m *ast.FuncType) string {
	returnType := "void"
	returnPrefix := ""
	if m.Results != nil && len(m.Results.List) != 0 {
		if len(m.Results.List) != 1 || len(m.Results.List[0].Names) != 0 {
			log.Fatalf("%s: only one result allowed", name)
		}
		returnType = g.getCType(exprToString(m.Results.List[0].Type))
		returnPrefix = "return "
	}

	var paramDecls []string
	var typedefParams []string
	var callArgs []string
	vargs_param := ""

	paramDecls = append(paramDecls, "void* f")
	for _, p := range m.Params.List {
		n := p.Names[0].String()
		t := g.getCType(exprToString(p.Type))
		if strings.HasSuffix(n, "_vargs") {
			paramDecls = append(paramDecls, fmt.Sprintf("int %s_len", n), fmt.Sprintf("%s %s", t, n))
			typedefParams = append(typedefParams, "...")
			vargs_param = n
		} else {
			paramDecls = append(paramDecls, fmt.Sprintf("%s %s", t, n))
			typedefParams = append(typedefParams, t)
			callArgs = append(callArgs, n)
		}
	}

	funcStr := fmt.Sprintf("%s _trampoline_%s(%s) {\n", returnType, name, strings.Join(paramDecls, ", "))
	body := fmt.Sprintf("typedef %s (*F)(%s);\n", returnType, strings.Join(typedefParams, ", "))
	if vargs_param == "" {
		body += fmt.Sprintf("%s((F)f)(%s);\n", returnPrefix, strings.Join(callArgs, ", "))
	} else {
		body += fmt.Sprintf("switch(%s_len) {\n", vargs_param)
		for i := 0; i < 10; i++ {
			body += fmt.Sprintf("case %d: %s((F)f)(%s);\n", i, returnPrefix, strings.Join(callArgs, ", "))
			callArgs = append(callArgs, fmt.Sprintf("%s[%d]", vargs_param, i))
		}
		body += "default: assert(0);\n"
		body += "}\n"
	}
	funcStr += indent(body, 1)
	funcStr += "}\n"
	return funcStr
}

func (g *generator) getCType(t string) string {
	switch t {
	case "int", "ssize_t":
		return t
	case "string":
		return "char*"
	case "unsafe.Pointer":
		return "void*"
	}
	if strings.HasPrefix(t, "[]") {
		return "void**"
	}
	if strings.HasPrefix(t, "*") {
		return "void*"
	}
	log.Fatalf("unknown type %s", t)
	return ""
}

func indent(s string, i int) string {
	l := strings.Split(s, "\n")
	is := strings.Repeat("\t", i)
	for i, _ := range l {
		if l[i] != "" {
			l[i] = is + l[i]
		}
	}
	return strings.Join(l, "\n")
}
