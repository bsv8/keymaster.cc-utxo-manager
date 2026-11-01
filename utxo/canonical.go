package utxo

import (
	"bytes"
	"sort"
	"strconv"
	"unicode/utf16"
)

// JS strings compare UTF-16 code units, not UTF-8 bytes or Unicode scalars.
func jsLess(a, b string) bool {
	aa, bb := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(aa) && i < len(bb); i++ {
		if aa[i] != bb[i] {
			return aa[i] < bb[i]
		}
	}
	return len(aa) < len(bb)
}
func sortJSStrings(values []string) {
	sort.Slice(values, func(i, j int) bool { return jsLess(values[i], values[j]) })
}

// JSON.stringify enumerates object array-index keys first in numeric order;
// the remaining keys here were inserted by the TS kernel's UTF-16 sort.
type jsObject[T any] map[string]T

func (object jsObject[T]) MarshalJSON() ([]byte, error) {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	index := func(key string) (uint64, bool) {
		value, err := strconv.ParseUint(key, 10, 32)
		return value, err == nil && value < 0xffffffff && strconv.FormatUint(value, 10) == key
	}
	sort.Slice(keys, func(i, j int) bool {
		a, ai := index(keys[i])
		b, bi := index(keys[j])
		if ai != bi {
			return ai
		}
		if ai {
			return a < b
		}
		return jsLess(keys[i], keys[j])
	})
	var out bytes.Buffer
	out.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		name, err := marshalCanonical(key)
		if err != nil {
			return nil, err
		}
		value, err := marshalCanonical(object[key])
		if err != nil {
			return nil, err
		}
		out.WriteString(name)
		out.WriteByte(':')
		out.WriteString(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// encoding/json escapes these separators even with EscapeHTML(false), whereas
// JSON.stringify emits the literal characters. Escaped backslashes must stay
// escaped: a string containing the six characters "\\u2028" is not U+2028.
func jsJSONSeparators(value string) string {
	var out bytes.Buffer
	for i := 0; i < len(value); {
		if value[i] == '\\' && i+1 < len(value) {
			if i+6 <= len(value) && (value[i:i+6] == `\u2028` || value[i:i+6] == `\u2029`) {
				if value[i+5] == '8' {
					out.WriteRune('\u2028')
				} else {
					out.WriteRune('\u2029')
				}
				i += 6
			} else {
				out.WriteString(value[i : i+2])
				i += 2
			}
		} else {
			out.WriteByte(value[i])
			i++
		}
	}
	return out.String()
}
