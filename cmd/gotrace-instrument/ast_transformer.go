package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ASTTransformer struct {
	FileSet     *token.FileSet
	AddTrace    bool
	AddLogging  bool
	Verbose     bool
	modified    bool
	hasDevtrace bool
	packageName string
	fileName    string
}

func (t *ASTTransformer) Transform(file *ast.File) bool {
	t.modified = false
	t.hasDevtrace = false
	t.packageName = file.Name.Name

	if pos := t.FileSet.Position(file.Pos()); pos.IsValid() {
		t.fileName = filepath.Base(pos.Filename)
	}

	// Check if devtrace is already imported
	t.checkExistingImports(file)

	// Visit all nodes in the AST
	ast.Inspect(file, t.visit)

	// Add devtrace import if we made modifications and it's not already imported
	if t.modified && !t.hasDevtrace {
		t.addDevtraceImport(file)
	}

	return t.modified
}

func (t *ASTTransformer) checkExistingImports(file *ast.File) {
	for _, imp := range file.Imports {
		if imp.Path.Value == `"github.com/hackathon/gotrace"` {
			t.hasDevtrace = true
			break
		}
	}
}

func (t *ASTTransformer) addDevtraceImport(file *ast.File) {
	// Create new import spec
	importSpec := &ast.ImportSpec{
		Name: ast.NewIdent("devtrace"),
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"github.com/hackathon/gotrace"`,
		},
	}

	// Find or create import declaration
	var importDecl *ast.GenDecl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl
			break
		}
	}

	if importDecl == nil {
		// Create new import declaration
		importDecl = &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{importSpec},
		}

		// Insert at the beginning of declarations
		newDecls := make([]ast.Decl, len(file.Decls)+1)
		newDecls[0] = importDecl
		copy(newDecls[1:], file.Decls)
		file.Decls = newDecls
	} else {
		// Add to existing import declaration
		importDecl.Specs = append(importDecl.Specs, importSpec)
	}

	if t.Verbose {
		log.Printf("Added devtrace import to %s", t.fileName)
	}
}

func (t *ASTTransformer) visit(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.FuncDecl:
		if t.AddTrace {
			t.instrumentFunction(n)
		}
	case *ast.CallExpr:
		if t.AddLogging {
			t.instrumentLogCall(n)
		}
	}
	return true
}

func (t *ASTTransformer) instrumentFunction(fn *ast.FuncDecl) {
	// Skip functions that are already instrumented or shouldn't be instrumented
	if t.shouldSkipFunction(fn) {
		return
	}

	// Skip functions without body (interfaces, etc.)
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return
	}

	functionName := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		// Method - include receiver type
		if field := fn.Recv.List[0]; field.Type != nil {
			typeName := t.getTypeName(field.Type)
			functionName = typeName + "." + functionName
		}
	}

	// Get position information
	pos := t.FileSet.Position(fn.Pos())

	// Create arguments map for tracing
	argsMap := t.createArgsMapForFunction(fn)

	signature := t.buildSignatureForFunction(fn)

	// Create the frame creation statement
	frameStmt := t.createFrameStatement(functionName, signature, pos.Line, argsMap)

	// Create defer statement for leaving the trace
	deferStmt := &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("devtrace"),
				Sel: ast.NewIdent("GlobalLeave"),
			},
		},
	}

	// Add statements to the beginning of function body
	newStmts := make([]ast.Stmt, 0, len(fn.Body.List)+2)
	newStmts = append(newStmts, frameStmt, deferStmt)
	newStmts = append(newStmts, fn.Body.List...)
	fn.Body.List = newStmts

	t.modified = true

	if t.Verbose {
		log.Printf("Instrumented function: %s in %s:%d", functionName, t.fileName, pos.Line)
	}
}

func (t *ASTTransformer) shouldSkipFunction(fn *ast.FuncDecl) bool {
	name := fn.Name.Name

	// Skip init functions
	if name == "init" {
		return true
	}

	// Skip main function in main package
	if t.packageName == "main" && name == "main" {
		return true
	}

	// Skip test functions
	if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example") {
		return true
	}

	// Skip functions that start with underscore (private/internal)
	if strings.HasPrefix(name, "_") {
		return true
	}

	return false
}

func (t *ASTTransformer) createArgsMapForFunction(fn *ast.FuncDecl) *ast.CompositeLit {
	var elts []ast.Expr

	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				// Create key-value pair for the map
				kvExpr := &ast.KeyValueExpr{
					Key:   &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(name.Name)},
					Value: name,
				}
				elts = append(elts, kvExpr)
			}
		}
	}

	return &ast.CompositeLit{
		Type: &ast.MapType{
			Key: &ast.Ident{Name: "string"},
			Value: &ast.InterfaceType{
				Methods: &ast.FieldList{},
			},
		},
		Elts: elts,
	}
}

func (t *ASTTransformer) createFrameStatement(functionName, signature string, line int, argsMap *ast.CompositeLit) ast.Stmt {
	// Create: devtrace.GlobalEnter(devtrace.CreateFrame("functionName", "signature", "filename", line, argsMap))
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("devtrace"),
				Sel: ast.NewIdent("GlobalEnter"),
			},
			Args: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   ast.NewIdent("devtrace"),
						Sel: ast.NewIdent("CreateFrame"),
					},
					Args: []ast.Expr{
						&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(functionName)},
						&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(signature)},
						&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(t.fileName)},
						&ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(line)},
						argsMap,
					},
				},
			},
		},
	}
}

func (t *ASTTransformer) buildSignatureForFunction(fn *ast.FuncDecl) string {
	var builder strings.Builder
	builder.WriteString(fn.Name.Name)
	builder.WriteString("(")

	params := make([]string, 0)
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			typeStr := t.renderExpr(field.Type)
			if len(field.Names) == 0 {
				params = append(params, typeStr)
				continue
			}
			for _, name := range field.Names {
				params = append(params, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		}
	}
	builder.WriteString(strings.Join(params, ", "))
	builder.WriteString(")")

	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := make([]string, 0)
		for _, field := range fn.Type.Results.List {
			typeStr := t.renderExpr(field.Type)
			if len(field.Names) == 0 {
				results = append(results, typeStr)
				continue
			}
			for _, name := range field.Names {
				results = append(results, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		}

		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			builder.WriteString(" ")
			builder.WriteString(results[0])
		} else {
			builder.WriteString(" (")
			builder.WriteString(strings.Join(results, ", "))
			builder.WriteString(")")
		}
	}

	return builder.String()
}

func (t *ASTTransformer) renderExpr(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, t.FileSet, expr); err != nil {
		return ""
	}

	return buf.String()
}

func (t *ASTTransformer) instrumentLogCall(call *ast.CallExpr) {
	// Check if this is a log call (log.Print, log.Printf, etc.)
	if !t.isLogCall(call) {
		return
	}

	// Skip if already instrumented
	if t.isAlreadyInstrumentedLog(call) {
		return
	}

	// Add devtrace enhanced logging
	// Transform log.Print(args...) to devtrace.GlobalEnhancedLogger.Info(context.Background(), msg, args...)
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		// Change the call to use devtrace enhanced logger
		call.Fun = &ast.SelectorExpr{
			X: &ast.SelectorExpr{
				X:   ast.NewIdent("devtrace"),
				Sel: ast.NewIdent("GlobalEnhancedLogger"),
			},
			Sel: ast.NewIdent("Info"),
		}

		// Add context.Background() as first argument
		contextCall := &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("context"),
				Sel: ast.NewIdent("Background"),
			},
		}

		// Prepend context to arguments
		newArgs := make([]ast.Expr, 0, len(call.Args)+1)
		newArgs = append(newArgs, contextCall)
		newArgs = append(newArgs, call.Args...)
		call.Args = newArgs

		t.modified = true

		if t.Verbose {
			log.Printf("Instrumented log call in %s", t.fileName)
		}
	}
}

func (t *ASTTransformer) isLogCall(call *ast.CallExpr) bool {
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selector.X.(*ast.Ident); ok {
			return ident.Name == "log" && (selector.Sel.Name == "Print" ||
				selector.Sel.Name == "Printf" ||
				selector.Sel.Name == "Println" ||
				selector.Sel.Name == "Fatal" ||
				selector.Sel.Name == "Fatalf" ||
				selector.Sel.Name == "Fatalln" ||
				selector.Sel.Name == "Panic" ||
				selector.Sel.Name == "Panicf" ||
				selector.Sel.Name == "Panicln")
		}
	}
	return false
}

func (t *ASTTransformer) isAlreadyInstrumentedLog(call *ast.CallExpr) bool {
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		if nestedSelector, ok := selector.X.(*ast.SelectorExpr); ok {
			if ident, ok := nestedSelector.X.(*ast.Ident); ok {
				return ident.Name == "devtrace"
			}
		}
	}
	return false
}

func (t *ASTTransformer) getTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + t.getTypeName(e.X)
	case *ast.SelectorExpr:
		return t.getTypeName(e.X) + "." + e.Sel.Name
	default:
		return "Unknown"
	}
}

func (t *ASTTransformer) WriteFile(outputPath string, file *ast.File) error {
	// Create output directory if it doesn't exist
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Open output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %v", outputPath, err)
	}
	defer outFile.Close()

	// Format and write the AST
	if err := format.Node(outFile, t.FileSet, file); err != nil {
		return fmt.Errorf("failed to write formatted code to %s: %v", outputPath, err)
	}

	if t.Verbose {
		log.Printf("Written instrumented code to: %s", outputPath)
	}

	return nil
}
