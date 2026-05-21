package main

import "os"

func helper() {
	os.Exit(2) // вызов вне main — не должен триггерить анализатор
}

func main() {
	os.Exit(1) // want "прямой вызов os\\.Exit в функции main запрещён"
}
