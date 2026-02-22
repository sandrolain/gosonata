package fuzz

import (
	"context"
	"testing"
	"time"

	"github.com/sandrolain/gosonata"
)

var fixtureData = map[string]interface{}{
	"name": "Alice",
	"age":  float64(30),
	"items": []interface{}{
		map[string]interface{}{"name": "foo", "price": float64(10)},
		map[string]interface{}{"name": "bar", "price": float64(200)},
	},
}

func FuzzEvaluator(f *testing.F) {
	seeds := []string{
		`$.name`,
		`$.items[price > 100].name`,
		`$sum($.items.price)`,
		`$count($.items)`,
		`$string($.age)`,
		`$type($.age)`,
		`$keys($)`,
		`1/0`,
		`$.missing.path`,
		``,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, _ = gosonata.EvalWithContext(ctx, input, fixtureData)
	})
}
