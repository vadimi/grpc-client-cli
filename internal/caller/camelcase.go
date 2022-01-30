package caller

import (
	"strings"
	"unicode"
)

func toLowerCamelCase(s string) string {
	res := strings.Builder{}
	capNext := false
	for _, v := range []byte(s) {
		if v >= 'A' && v <= 'Z' {
			res.WriteByte(v)
		}
		if v >= '0' && v <= '9' {
			res.WriteByte(v)
		}
		if v >= 'a' && v <= 'z' {
			if capNext {
				res.WriteRune(unicode.ToUpper(rune(v)))
			} else {
				res.WriteByte(v)
			}
		}

		capNext = false
		if v == '_' || v == ' ' || v == '-' {
			capNext = true
		}
	}
	return res.String()
}
