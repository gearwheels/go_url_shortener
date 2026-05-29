package main

import myos "os"

func main() {
	myos.Exit(1) // want "прямой вызов os\\.Exit в функции main запрещён"
}
