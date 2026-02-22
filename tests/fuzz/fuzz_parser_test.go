package fuzz

import (
	"testing"

	"github.com/sandrolain/gosonata/pkg/parser"
)

func FuzzParser(f *testing.F) {
	seeds := []string{
		`$.name`,
		`$.items[price > 100]`,
		`$sum($.prices)`,
		`$map($.items, function($v) { $v.price * 2 })`,
		`$`,
		`$$`,
		`1 + 2 * 3`,
		``,
		`(`,
		`$foo(`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = parser.Compile(input)
	})
}
