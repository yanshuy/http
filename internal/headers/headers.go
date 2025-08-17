package headers

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
)

var Crlf = []byte("\r\n")
var CrlfLen = len(Crlf)

type Headers map[string][]string

func NewHeaders() Headers {
	return make(Headers)
}

func (h Headers) Get(key string) (string, bool) {
	vals, ok := h[strings.ToLower(key)]
	str := strings.Join(vals, ",")
	return str, ok
}

func (h Headers) GetTest(key string) string {
	vals := h[strings.ToLower(key)]
	str := strings.Join(vals, ",")
	return str
}

func (h Headers) Add(key, val string) {
	lower := strings.ToLower(key)
	h[lower] = append(h[lower], val)
}

func (h Headers) Set(key, val string) {
	lower := strings.ToLower(key)
	h[lower] = []string{val}
}

func (h Headers) Del(key string) {
	lower := strings.ToLower(key)
	delete(h, lower)
}

// TODO: use this when parsing
func (h Headers) Parse(data []byte) (n int, done bool, err error) {
	read := 0
	for {
		i := bytes.Index(data, Crlf)
		if i == -1 {
			return read, false, nil
		}
		if i == 0 {
			read += CrlfLen
			break
		}
		err := h.ParseHearderLine(data[:i])
		if err != nil {
			return 0, false, err
		}
		lineLen := i + CrlfLen
		data = data[lineLen:]
		read += lineLen
	}
	return read, true, nil
}

func (h Headers) ParseHearderLine(line []byte) (err error) {
	parts := bytes.SplitN(line, []byte(":"), 2)
	if len(parts) != 2 {
		return ErrMalformedRequestHeader
	}

	key := bytes.TrimLeftFunc(parts[0], unicode.IsSpace)
	val := bytes.TrimSpace(parts[1])

	space := bytes.ContainsFunc(key, unicode.IsSpace)
	if space {
		return ErrMalformedRequestHeader
	}

	// TODO: validate field name
	// Uppercase letters: A-Z
	// Lowercase letters: a-z
	// Digits: 0-9
	// Special characters: !, #, $, %, &, ', *, +, -, ., ^, _, `, |, ~
	// Multiple header lines with the same key should append.
	h.Add(string(key), string(val))
	return nil
}

var ErrMalformedRequestHeader = errors.New("malformed request header")
