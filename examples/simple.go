// Simple example demonstrating GoSonata usage
// TODO: Implement after Phase 4 (Evaluator)

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sandrolain/gosonata"
)

func main() {
	// Sample JSON data
	dataJSON := `{
		"name": "John Doe",
		"age": 30,
		"email": "john@example.com",
		"items": [
			{"name": "Item 1", "price": 50},
			{"name": "Item 2", "price": 150},
			{"name": "Item 3", "price": 75}
		]
	}`

	var data interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		log.Fatal(err)
	}

	// Example 1: Simple field access
	fmt.Println("Example 1: Get name")
	result, err := gosonata.Eval("$.name", data)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %v\n\n", result)
	}

	// Example 2: Filter items
	fmt.Println("Example 2: Get items with price > 100")
	result, err = gosonata.Eval("$.items[price > 100]", data)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Result: %s\n\n", resultJSON)
	}

	// Example 3: Compile once, evaluate many times
	fmt.Println("Example 3: Compiled expression")
	expr, err := gosonata.Compile("$.items.price")
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err = expr.Eval(ctx, data)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %v\n\n", result)
	}

	// Example 4: With options
	fmt.Println("Example 4: With options")
	// Note: This will not work until Phase 4 is implemented
	fmt.Println("(Implementation pending)")
}
