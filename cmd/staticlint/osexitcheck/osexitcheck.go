// Package osexitcheck реализует анализатор, запрещающий прямой вызов os.Exit
// в функции main пакета main.
//
// # Мотивация
//
// Прямой вызов os.Exit в main() обходит все отложенные (defer) вызовы,
// делает код нетестируемым и затрудняет корректное завершение ресурсов.
// Вместо этого следует возвращаться из main() или вызывать os.Exit
// из вспомогательной функции, которую можно подменить в тестах.
//
// # Пример нарушения
//
//	package main
//
//	import "os"
//
//	func main() {
//	    os.Exit(1) // ошибка: прямой вызов os.Exit в main запрещён
//	}
//
// # Корректный вариант
//
//	package main
//
//	import (
//	    "log/slog"
//	    "net/http"
//	)
//
//	func main() {
//	    if err := http.ListenAndServe(":8080", nil); err != nil {
//	        slog.Error("server error", "err", err)
//	        // просто return — deferred вызовы сработают, ресурсы освободятся
//	    }
//	}
package osexitcheck

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer — анализатор, запрещающий os.Exit в функции main пакета main.
var Analyzer = &analysis.Analyzer{
	Name:     "osexitcheck",
	Doc:      "запрещает прямой вызов os.Exit в функции main пакета main",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	// Пропускаем автогенерированные main-файлы тестового фреймворка.
	// Такие файлы находятся в кэше сборки Go или заканчиваются на _testmain.go.
	if isGeneratedTestMain(pass) {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" || fn.Recv != nil || fn.Body == nil {
			return
		}

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkg.Name == "os" && sel.Sel.Name == "Exit" {
				pass.Reportf(call.Pos(), "прямой вызов os.Exit в функции main запрещён")
			}
			return true
		})
	})

	return nil, nil
}

// isGeneratedTestMain возвращает true, если анализируемый пакет — это
// автогенерированный тестовый main (например, _testmain.go из кэша сборки).
func isGeneratedTestMain(pass *analysis.Pass) bool {
	cacheDir, _ := os.UserCacheDir()
	goBuildCache := os.Getenv("GOCACHE")

	for _, f := range pass.Files {
		pos := pass.Fset.Position(f.Pos())
		fname := pos.Filename

		if strings.HasSuffix(fname, "_testmain.go") {
			return true
		}

		// Файл находится в кэше сборки Go
		if cacheDir != "" && strings.HasPrefix(filepath.ToSlash(fname), filepath.ToSlash(cacheDir)) {
			return true
		}
		if goBuildCache != "" && strings.HasPrefix(filepath.ToSlash(fname), filepath.ToSlash(goBuildCache)) {
			return true
		}
	}
	return false
}
