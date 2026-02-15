package main

import (
	"fmt"
	"log"

	"github.com/sandrolain/gosonata"
)

func main() {
	// Test $distinct
	fmt.Println("=== Testing $distinct ===")
	result, err := gosonata.Eval(`$distinct([1, 2, 2, 3, 1, 4])`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("$distinct([1, 2, 2, 3, 1, 4]) = %v\n", result)

	// Test $shuffle
	fmt.Println("\n=== Testing $shuffle ===")
	result, err = gosonata.Eval(`$shuffle([1, 2, 3, 4, 5])`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("$shuffle([1, 2, 3, 4, 5]) = %v\n", result)

	// Test $zip
	fmt.Println("\n=== Testing $zip ===")
	result, err = gosonata.Eval(`$zip([1, 2, 3], ["a", "b", "c"])`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("$zip([1, 2, 3], [\"a\", \"b\", \"c\"]) = %v\n", result)

	// Test $pad
	fmt.Println("\n=== Testing $pad ===")
	result, err = gosonata.Eval(`$pad("hello", 10)`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("$pad(\"hello\", 10) = '%v'\n", result)

	result, err = gosonata.Eval(`$pad("hello", -10, "*")`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("$pad(\"hello\", -10, \"*\") = '%v'\n", result)

	// Test $substringBefore
	fmt.Println("\n=== Testing $substringBefore ===")
	result, err = gosonata.Eval(`$substringBefore("hello world", " ")`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("$substringBefore(\"hello world\", \" \") = '%v'\n", result)

	// Test $substringAfter
	fmt.Println("\n=== Testing $substringAfter ===")
	result, err = gosonata.Eval(`$substringAfter("hello world", " ")`, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("$substringAfter(\"hello world\", \" \") = '%v'\n", result)

	fmt.Println("\n=== All Fase 5.2 functions tested successfully! ===")
}
