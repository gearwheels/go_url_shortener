package main

import (
	"log"
	"os"
)

func helper() {
	os.Exit(2)      // вызов вне main — не должен триггерить анализатор
	log.Fatal("msg") // вызов log.Fatal вне main — не должен триггерить анализатор
}

func main() {
	os.Exit(1)      // want "прямой вызов os\\.Exit в функции main запрещён"
	log.Fatal("x")  // want "вызов log\\.Fatal в функции main запрещён"
	panic("x")      // want "вызов panic в функции main запрещён"
}
