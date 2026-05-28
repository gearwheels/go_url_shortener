package notmain

import "os"

// Exit в не-main пакете — анализатор не должен реагировать.
func Shutdown(code int) {
	os.Exit(code)
}
