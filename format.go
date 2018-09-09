package main

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var BadFormatStr = fmt.Errorf("bad format string")

func FormatHelp(keys []KeyDesc) string {
	return `
<fmtstr> syntax:
  $<var>:   The value of a preset or user-set variable. Must be followed
            by something other than a letter, number or _.
            Will error if the variable is not defined without a default value.
  ${<var>}: Alternative syntax.
  [...]:    Only displays the text if all variables used inside are defined.
            For example to only display the '::' if there are unmet deps. use:
               $path[ :: $unmet]
  \...:     Standard backslash escaping.

preset variables:
` + KeysHelp(keys)
}

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
			if len(str) >= 2 && asciiIsSymbol(str[1]) {
				buf.WriteByte(str[1])
				str = str[2:]
			} else {
				ch, _, s, e := strconv.UnquoteChar(str, 0)
				if e != nil {
					err = BadFormatStr
					return
				}
				str = s
				buf.WriteRune(ch)
			}
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
					err = BadFormatStr
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

func asciiIsSymbol(ch byte) bool {
	return '!' <= ch && ch <= '/' || ':' <= ch && ch <= '@' || '[' <= ch && ch <= '`' || '{' <= ch && ch <= '~'
}
