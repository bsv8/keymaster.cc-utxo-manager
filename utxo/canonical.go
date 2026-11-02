package utxo

import (
	"sort"
	"unicode/utf16"
)

// Match JavaScript string ordering in pure transaction helpers.
func sortJSStrings(values []string) {
	sort.Slice(values, func(i, j int) bool {
		a, b := utf16.Encode([]rune(values[i])), utf16.Encode([]rune(values[j]))
		for n := 0; n < len(a) && n < len(b); n++ {
			if a[n] != b[n] {
				return a[n] < b[n]
			}
		}
		return len(a) < len(b)
	})
}
