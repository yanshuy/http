package request

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/yanshuy/http/internal/headers"
)

var Crlf = []byte("\r\n")
var CrlfLen = len(Crlf)

type RequestLine struct {
	Method      string
	Target      string
	HttpVersion string
}

type Request struct {
	*RequestLine
	headers.Headers
	Body []byte
}

func NewRequest() *Request {
	return &Request{
		Headers: headers.NewHeaders(),
	}
}

type parseState int

const (
	StateStart parseState = iota
	StateHeaders
	StateHeadersDone
	StateBody
	StateDone
)

type RequestParser struct {
	*Request
	currentPos int
	state      parseState
}

func NewRequestParser() *RequestParser {
	return &RequestParser{
		Request:    NewRequest(),
		currentPos: 0,
		state:      StateStart,
	}
}

func (rp *RequestParser) Done() bool {
	return rp.state == StateDone
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	rp := NewRequestParser()
	buf := make([]byte, 4096)
	bufLen := 0
	for !rp.Done() {
		n, err := reader.Read(buf[bufLen:])

		if n > 0 {
			bufLen += n
			readN, err := rp.parse(buf[:bufLen])
			if err != nil {
				return nil, err
			}

			if readN > 0 {
				copy(buf, buf[readN:bufLen])
				bufLen -= readN
			}
		}

		if err != nil {
			return nil, fmt.Errorf("unexpected %w", err)
		}
	}

	return rp.Request, nil
}

func (rp *RequestParser) parse(data []byte) (int, error) {
	for {
		// slog.Info("StateBody", "body", string(rp.Body), "data", string(data[rp.currentPos:]), "state", rp.state)
		switch rp.state {
		case StateStart:
			i := bytes.Index(data, Crlf)
			if i == -1 {
				return 0, nil
			}
			reqline, err := parseRequestLine(data[:i])
			if err != nil {
				return 0, err
			}
			rp.currentPos = i + CrlfLen
			rp.RequestLine = reqline
			rp.state = StateHeaders

		case StateHeaders:
			i := bytes.Index(data[rp.currentPos:], Crlf)
			if i == -1 {
				return 0, nil
			}
			if i == 0 {
				rp.currentPos += CrlfLen
				rp.state = StateHeadersDone
				continue
			}
			err := rp.Headers.ParseHearderLine(data[rp.currentPos : rp.currentPos+i])
			if err != nil {
				return 0, err
			}
			lineLen := i + CrlfLen
			rp.currentPos += lineLen

		case StateHeadersDone:
			contLenStr, ok := rp.Headers.Get("Content-Length")
			if !ok {
				rp.state = StateDone
				continue
			}
			contLen, err := strconv.ParseInt(contLenStr, 10, 64)
			if err != nil {
				return 0, ErrInvalidContentLength
			}
			if contLen == 0 {
				rp.state = StateDone
				continue
			}
			rp.Body = make([]byte, 0, contLen)
			rp.state = StateBody

		case StateBody:
			if len(data[rp.currentPos:]) == 0 {
				return 0, nil
			}

			remaining := cap(rp.Body) - len(rp.Body)
			if len(data[rp.currentPos:]) > remaining {
				return 0, ErrBodyGreaterThanContentLength
			}
			rp.Body = append(rp.Body, data[rp.currentPos:]...)
			rp.currentPos += len(data[rp.currentPos:])

			if cap(rp.Body) == len(rp.Body) {
				rp.state = StateDone
			}

		case StateDone:
			return rp.currentPos, nil
		}
	}
}

func parseRequestLine(line []byte) (*RequestLine, error) {
	parts := bytes.Split(line, []byte(" "))
	if len(parts) < 3 {
		return nil, ErrMalformedRequestLine
	}

	httpVersion := strings.Split(string(parts[2]), "/")
	if httpVersion[0] != "HTTP" {
		return nil, ErrMalformedRequestLine
	}
	if !IsVersionSupported(httpVersion[1]) {
		return nil, ErrUnsupportedVersion
	}

	return &RequestLine{
		Method:      string(parts[0]),
		Target:      string(parts[1]),
		HttpVersion: httpVersion[1],
	}, nil
}

var ErrMalformedRequestLine = errors.New("malformed request line")
var ErrUnsupportedVersion = errors.New("version not supported")
var ErrInvalidContentLength = errors.New("invalid content length")
var ErrBodyGreaterThanContentLength = errors.New("body greater than the content length")

func IsVersionSupported(httpVersion string) bool {
	return httpVersion == "1.1"
}
