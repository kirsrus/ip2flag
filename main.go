package main

import "fmt"

// Версия программы из Git. Передаётся при компиляции программы.
var version string = "v0.0.0"

func main() {
	fmt.Println("Hello!", version)
}
