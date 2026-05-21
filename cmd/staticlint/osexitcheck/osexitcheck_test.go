package osexitcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/gearwheels/go_url_shortener/cmd/staticlint/osexitcheck"
)

// TestOsExitCheck_WithExit проверяет, что анализатор сообщает об os.Exit в main().
func TestOsExitCheck_WithExit(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), osexitcheck.Analyzer, "main_with_exit")
}

// TestOsExitCheck_NoExit проверяет, что анализатор молчит, когда os.Exit не вызывается.
func TestOsExitCheck_NoExit(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), osexitcheck.Analyzer, "main_no_exit")
}

// TestOsExitCheck_NotMainPackage проверяет, что анализатор игнорирует
// os.Exit в пакетах, отличных от main.
func TestOsExitCheck_NotMainPackage(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), osexitcheck.Analyzer, "notmain")
}
