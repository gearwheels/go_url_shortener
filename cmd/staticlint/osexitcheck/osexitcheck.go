// Package osexitcheck реализует анализатор, запрещающий прямой вызов os.Exit,
// log.Fatal* и panic в функции main пакета main.
//
// # Мотивация
//
// Прямой вызов os.Exit/log.Fatal в main() обходит все отложенные (defer) вызовы,
// делает код нетестируемым и затрудняет корректное завершение ресурсов.
// panic в main() создаёт непредсказуемый вывод вместо аккуратного завершения.
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

// Analyzer — анализатор, запрещающий os.Exit, log.Fatal* и panic в функции main пакета main.
var Analyzer = &analysis.Analyzer{
	Name:     "osexitcheck",
	Doc:      "запрещает вызовы os.Exit, log.Fatal* и panic в функции main пакета main",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	// Пропускаем автогенерированные main-файлы тестового фреймворка.
	if isGeneratedTestMain(pass) {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" || fn.Recv != nil || fn.Body == nil {
			return
		}

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			switch fun := call.Fun.(type) {
			case *ast.Ident:
				// Проверяем встроенный panic через TypesInfo: у builtin-объектов Pkg() == nil.
				if fun.Name == "panic" {
					if obj := pass.TypesInfo.Uses[fun]; obj != nil && obj.Pkg() == nil {
						pass.Reportf(call.Pos(), "вызов panic в функции main запрещён")
					}
				}

			case *ast.SelectorExpr:
				// Используем TypesInfo.Uses для корректной работы с алиасами импортов.
				obj := pass.TypesInfo.Uses[fun.Sel]
				if obj == nil || obj.Pkg() == nil {
					return true
				}
				pkgPath := obj.Pkg().Path()
				name := obj.Name()
				switch {
				case pkgPath == "os" && name == "Exit":
					pass.Reportf(call.Pos(), "прямой вызов os.Exit в функции main запрещён")
				case pkgPath == "log" && (name == "Fatal" || name == "Fatalf" || name == "Fatalln"):
					pass.Reportf(call.Pos(), "вызов log.Fatal в функции main запрещён")
				}
			}

			return true
		})
	})

	return nil, nil
}

// isGeneratedTestMain возвращает true, если анализируемый пакет — это
// автогенерированный тестовый main (например, _testmain.go из кэша сборки).
func isGeneratedTestMain(pass *analysis.Pass) bool {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = ""
	}
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
