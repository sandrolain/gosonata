// GoSonata examples — demonstrates the main features of the library.
//
// Run with:
//
//	go run ./examples/simple.go

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sandrolain/gosonata"
	"github.com/sandrolain/gosonata/pkg/evaluator"
)

// ── helpers ─────────────────────────────────────────────────────────────────

// fromJSON parses raw JSON into interface{} so that all numbers become
// float64 (matching JSONata JS behaviour). Always use this with data that
// contains numeric fields used in comparisons or arithmetic.
func fromJSON(raw string) interface{} {
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		log.Fatalf("fromJSON: %v", err)
	}
	return v
}

func printResult(label string, result interface{}, err error) {
	fmt.Printf("  %-44s ", label+":")
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	if result == nil {
		fmt.Println("<nil / undefined>")
		return
	}
	b, _ := json.Marshal(result)
	fmt.Println(string(b))
}

func section(title string) {
	fmt.Printf("\n── %s\n", title)
}

// ── sample data ──────────────────────────────────────────────────────────────

var catalogJSON = `{
	"store": "GoShop",
	"currency": "EUR",
	"products": [
		{"id": 1, "name": "Widget",       "price": 49.99,  "category": "tools", "stock": 120},
		{"id": 2, "name": "Gadget",       "price": 149.99, "category": "tech",  "stock": 3},
		{"id": 3, "name": "Doohickey",    "price": 9.99,   "category": "tools", "stock": 55},
		{"id": 4, "name": "Thingamajig",  "price": 299.0,  "category": "tech",  "stock": 0},
		{"id": 5, "name": "Whatnot",      "price": 74.50,  "category": "tools", "stock": 30}
	],
	"orders": [
		{"orderId": "A1", "customer": "Alice", "amount": 249.98, "status": "shipped"},
		{"orderId": "A2", "customer": "Bob",   "amount": 9.99,   "status": "pending"},
		{"orderId": "A3", "customer": "Alice", "amount": 299.0,  "status": "shipped"},
		{"orderId": "A4", "customer": "Carol", "amount": 74.50,  "status": "cancelled"}
	]
}`

func main() {
	catalog := fromJSON(catalogJSON)
	ctx := context.Background()

	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║              GoSonata — examples                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	// show is a shorthand for eval + printResult against catalog.
	show := func(label, query string, opts ...evaluator.EvalOption) {
		r, e := gosonata.Eval(query, catalog, opts...)
		printResult(label, r, e)
	}

	// ── 1. Simple field access ──────────────────────────────────────────────
	section("1. Simple field access")
	show("store name", "$.store")
	show("currency", "$.currency")
	show("all product names", "$.products.name")

	// ── 2. Filtering with predicates ────────────────────────────────────────
	section("2. Filtering with predicates")
	show("products price > 100", "$.products[price > 100].name")
	show("out-of-stock products", "$.products[stock = 0].name")
	show("shipped orders", `$.orders[status = "shipped"].orderId`)
	show("Alice's orders", `$.orders[customer = "Alice"].orderId`)

	// ── 3. Aggregation functions ────────────────────────────────────────────
	section("3. Aggregation functions")
	show("total product value", "$sum($.products.price)")
	show("number of products", "$count($.products)")
	show("average price", "$average($.products.price)")
	show("max price", "$max($.products.price)")
	show("min price", "$min($.products.price)")

	// ── 4. Object construction (projection) ─────────────────────────────────
	section("4. Object construction & projection")
	show("product summary", `
		$.products[price > 50].{
			"item":  name,
			"eur":   price,
			"avail": stock > 0
		}`)

	show("order totals by status", `{
		"shipped":   $sum($.orders[status = "shipped"].amount),
		"pending":   $sum($.orders[status = "pending"].amount),
		"cancelled": $sum($.orders[status = "cancelled"].amount)
	}`)

	// ── 5. Sorting ──────────────────────────────────────────────────────────
	// In JSONata $sort, the comparator returns true if $a should go AFTER $b.
	// So $a.price > $b.price → a after b when a is bigger → ascending order.
	section("5. Sorting")
	show("products by price asc",
		"$sort($.products, function($a,$b){$a.price > $b.price}).name")
	show("products by price desc",
		"$sort($.products, function($a,$b){$a.price < $b.price}).name")

	// ── 6. String functions ─────────────────────────────────────────────────
	section("6. String functions")
	show("uppercase store", "$uppercase($.store)")
	show("names joined", `$join($.products.name, " | ")`)
	show("substring of first name", "$substring($.products[0].name, 0, 3)")
	show("replace in store name", `$replace($.store, "Go", "Super")`)
	show("split store name", `$split("Go-Shop", "-")`)

	// ── 7. Array functions ──────────────────────────────────────────────────
	section("7. Array & object functions")
	show("distinct categories", "$distinct($.products.category)")
	show("reverse product names", "$reverse($.products.name)")
	show("keys of first order", "$keys($.orders[0])")
	show("spread first product", "$spread($.products[0])")

	// ── 8. Compile once, evaluate many times ────────────────────────────────
	section("8. Compile once, evaluate many times")
	expr, err := gosonata.Compile("$.products[price < $threshold].name")
	if err != nil {
		log.Fatalf("compile: %v", err)
	}

	ev := evaluator.New()
	for _, thr := range []float64{20.0, 80.0, 200.0} {
		res, evalErr := ev.EvalWithBindings(ctx, expr, catalog, map[string]interface{}{
			"threshold": thr,
		})
		printResult(fmt.Sprintf("price < %.0f", thr), res, evalErr)
	}

	// ── 9. Variable bindings ─────────────────────────────────────────────────
	section("9. Variable bindings")
	discountExpr, _ := gosonata.Compile(`
		$.products.{
			"name":  name,
			"final": $round(price * (1 - $discount), 2)
		}`)
	r, e := ev.EvalWithBindings(ctx, discountExpr, catalog, map[string]interface{}{
		"discount": 0.10,
	})
	printResult("products with 10% discount", r, e)

	// ── 10. Conditional (ternary) ────────────────────────────────────────────
	section("10. Conditional expressions")
	show("stock status labels", `
		$.products.{
			"name":   name,
			"status": stock > 0 ? "in stock" : "out of stock"
		}`)
	show("expensive or cheap", `
		$.products.{
			"name":  name,
			"tier":  price >= 100 ? "premium" :
			         price >= 50  ? "mid"     : "budget"
		}`)

	// ── 11. Chained operations ───────────────────────────────────────────────
	section("11. Chained operations")
	show("top 2 most expensive (names)",
		"$sort($.products, function($a,$b){$a.price < $b.price})[0..1].name")
	show("in-stock tech products",
		`$.products[category = "tech" and stock > 0].name`)
	show("count tools in stock",
		`$count($.products[category = "tools" and stock > 0])`)

	// ── 12. Type-checking functions ──────────────────────────────────────────
	section("12. Type functions")
	show("type of stock field", "$type($.products[0].stock)")
	show("type of category field", "$type($.products[0].category)")
	show("type of products array", "$type($.products)")
	show("number to string", `$string($.products[0].price)`)
	show("boolean to string", `$string($.products[0].stock > 0)`)

	// ── 13. WithTimeout option ───────────────────────────────────────────────
	section("13. Timeout option")
	show("sum with 5s timeout", "$sum($.products.price)",
		evaluator.WithTimeout(5*time.Second))

	// ── 14. Recursion depth guard (evaluator.WithMaxDepth) ─────────────────
	// evaluator.WithMaxDepth limits recursive lambda calls to prevent DoS.
	section("14. Recursion depth guard")
	fibExpr, _ := gosonata.Compile(`(
		$fib := function($n) { $n <= 1 ? $n : $fib($n-1) + $fib($n-2) };
		$fib(10)
	)`)
	fibResult, fibErr := evaluator.New().Eval(ctx, fibExpr, nil)
	printResult("fib(10) unlimited depth", fibResult, fibErr)

	_, depthErr := evaluator.New(evaluator.WithMaxDepth(5)).Eval(ctx, fibExpr, nil)
	fmt.Printf("  %-44s %v\n", "fib(10) with MaxDepth(5):", depthErr)

	// ── 15. Concurrent evaluation (goroutines) ───────────────────────────────
	section("15. Concurrent evaluation")
	queries := []struct{ label, query string }{
		{"count", "$count($.products)"},
		{"sum prices", "$sum($.products.price)"},
		{"max price", "$max($.products.price)"},
		{"min price", "$min($.products.price)"},
		{"avg price", "$average($.products.price)"},
	}

	concEv := evaluator.New(evaluator.WithConcurrency(true))
	type aggResult struct {
		label  string
		result interface{}
		err    error
	}
	ch := make(chan aggResult, len(queries))

	start := time.Now()
	for _, q := range queries {
		q := q
		go func() {
			compiled, _ := gosonata.Compile(q.query)
			res, evalErr := concEv.Eval(ctx, compiled, catalog)
			ch <- aggResult{q.label, res, evalErr}
		}()
	}
	for range queries {
		ag := <-ch
		printResult(ag.label, ag.result, ag.err)
	}
	fmt.Printf("  %-44s %v\n", "concurrent wall time:", time.Since(start))

	// ── 16. Error handling ───────────────────────────────────────────────────
	section("16. Error handling")
	_, syntaxErr := gosonata.Compile("$.items[price >>")
	fmt.Printf("  %-44s %v\n", "syntax error:", syntaxErr)

	notFound, _ := gosonata.Eval("$.nonexistent.deeply.nested", catalog)
	printResult("missing path returns nil", notFound, nil)

	// ── 17. MustCompile (panics on invalid expressions) ─────────────────────
	section("17. MustCompile")
	safeExpr := gosonata.MustCompile("$count($.orders)")
	cnt, _ := ev.Eval(ctx, safeExpr, catalog)
	printResult("order count via MustCompile", cnt, nil)

	fmt.Println("\nDone.")
}
