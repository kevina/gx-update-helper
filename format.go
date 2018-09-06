package main

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

var BadFormatStr = fmt.Errorf("bad format string")

func (v *Todo) Format(str string) ([]byte, error) {
	var buf bytes.Buffer
	i := 0
	for {
		j := strings.IndexByte(str[i:], '$')
		if j == -1 {
			buf.WriteString(str[i:])
			break
		}
		buf.WriteString(str[i : i+j])
		i += j
		i++
		if i >= len(str) {
			buf.WriteByte('$')
			break
		}
		key := ""
		switch str[i] {
		case '$':
			i++
		case '{':
			i++
			j = strings.IndexByte(str[i:], '}')
			if j == -1 {
				return nil, BadFormatStr
			}
			key = str[i : i+j]
			i += j + 1
		default:
			j := len(str[i:])
			for k, ch := range str[i:] {
				if ch != '_' && !unicode.IsLetter(ch) {
					j = k
					break
				}
			}
			key = str[i : i+j]
			i += j
		}
		if key == "" {
			buf.WriteByte('$')
		} else {
			val, _, err := v.Get(key)
			if err != nil {
				return nil, err
			}
			buf.WriteString(val)
		}
	}
	return buf.Bytes(), nil
}

