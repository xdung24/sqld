package main

import "fmt"

var (
	GoOs      string = "undefined"
	GoArch    string = "undefined"
	GitHash   string = "undefined"
	BuildTime string = "undefined"
)

func printInfo() {
	fmt.Printf("OS: %s \n", GoOs)
	fmt.Printf("Arch: %s \n", GoArch)
	fmt.Printf("Build: %s \n", GitHash)
	fmt.Printf("Build time: %s \n", BuildTime)
}
