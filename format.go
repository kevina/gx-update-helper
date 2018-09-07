package main

import (
	"fmt"
	"strings"
	"unicode"
	"strconv"
	"errors"
	"bytes"
)

var BadFormatStr = fmt.Errorf("bad format string")

func (v *Todo) Format(fmtorig string) ([]byte, error) {
	buf, fmt, err := v.format(fmtorig)
	if err != nil && err != defaultUsed {
		return nil, err
	}
	if len(fmt) != 0 {
		return nil, BadFormatStr
	}
	return buf.Bytes(), nil
}

var defaultUsed = errors.New("default used")

func (v *Todo) format(orig string) (buf bytes.Buffer, str string, err error) {
	str = orig
	for len(str) > 0 {
		switch str[0] {
		case '\\':
			ch, _, s, e := strconv.UnquoteChar(str, 0)
			if e != nil {
				err = e
				return
			}
			str = s
			buf.WriteRune(ch)
		case '[':
			b, s, e := v.format(str[1:])
			if e == BadFormatStr {
				err = e
				return
			}
			if e == nil {
				buf.Write(b.Bytes())
			}
			str = s
		case ']':
			str = str[1:]
			return
		case '$':
			key := ""
			str = str[1:]
			if len(str) == 0 {
				buf.WriteByte('$')
				return
			}
			if str[0] == '{' {
				i := strings.IndexByte(str, '}')
				if i == -1 {
					err =  BadFormatStr
					return
				}
				key = str[1:i]
				str = str[i+1:]
			} else {
				i := len(str)
				for j, ch := range str {
					if ch != '_' && !unicode.IsLetter(ch) {
						i = j
						break
					}
				}
				key = str[:i]
				str = str[i:]
			}
			if key == "" {
				buf.WriteByte('$')
				continue
			}
			val, ok, e := v.Get(key)
			if e != nil {
				err = e
				continue
			}
			if !ok {
				if err == nil {
					err = defaultUsed
				}
				continue
			}
			buf.WriteString(val)
		default:
			buf.WriteByte(str[0])
			str = str[1:]
		}
	}
	return
}
