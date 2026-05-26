package osexitcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/gearwheels/go_url_shortener/cmd/staticlint/osexitcheck"
)

// TestOsExitCheck_WithExit проверяет, что анализатор сообщает об os.Exit, log.Fatal и panic в main().
func TestOsExitCheck_WithExit(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), osexitcheck.Analyzer, "main_with_exit")
}

// TestOsExitCheck_NoExit проверяет, что анализатор молчит, когда запрещённые вызовы отсутствуют.
func TestOsExitCheck_NoExit(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), osexitcheck.Analyzer, "main_no_exit")
}

// TestOsExitCheck_NotMainPackage проверяет, что анализатор игнорирует
// os.Exit в пакетах, отличных от main.
func TestOsExitCheck_NotMainPackage(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), osexitcheck.Analyzer, "notmain")
}

// TestOsExitCheck_WithExitAlias проверяет, что анализатор обнаруживает os.Exit
// при использовании алиаса импорта (import myos "os").
func TestOsExitCheck_WithExitAlias(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), osexitcheck.Analyzer, "main_with_exit_alias")
}
