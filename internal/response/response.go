package response

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/yanshuy/http-server/internal/headers"
)

const ContentLength = "Content-Length"
const TransferEncoding = "Transfer-Encoding"

type Response struct {
	headers    headers.Headers
	statusCode int
	buf        []byte
}

func NewResponse(h headers.Headers) *Response {
	return &Response{
		headers:    h,
		statusCode: 200,
	}
}

type writeStatus int

const (
	StateInitial writeStatus = iota
	StateWroteStatus
	StateWroteHeader
	StateWroteBody
)

type Writer struct {
	*Response
	writer io.Writer
	writeStatus
	chunked    bool
	contentLen int
}

func NewResponseWriter(w io.Writer) *Writer {
	h := DefaultHeaders()
	resp := NewResponse(h)
	return &Writer{
		Response:    resp,
		writer:      w,
		writeStatus: StateInitial,
	}
}

func (w *Writer) Headers() headers.Headers {
	return w.headers
}

func (w *Writer) upgradeWriteStatus(ws writeStatus) error {
	for w.writeStatus < ws {
		switch w.writeStatus {
		case StateInitial:
			err := w.WriteStatus(200)
			if err != nil {
				return err
			}
		case StateWroteStatus:
			err := w.writeHeaders()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// transfer encoding chucked is used when content-length is not set
func (w *Writer) Write(p []byte) (int, error) {
	if err := w.upgradeWriteStatus(StateWroteHeader); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}

	if !w.chunked {
		return w.writeContent(p)
	}

	n, err := fmt.Fprintf(w.writer, "%x\r\n", len(p))
	if err != nil {
		return n, err
	}
	p = append(p, []byte("\r\n")...)
	o, err := w.writer.Write(p)
	if err != nil {
		return n + o, err
	}

	return len(p), nil
}

func (w *Writer) writeContent(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if len(w.buf) > w.contentLen {
		return 0, ErrWriteMoreThanContentLength
	}
	if len(w.buf) != w.contentLen {
		return len(p), nil
	}

	if n, err := w.writer.Write(w.buf); err != nil {
		return n, err
	}
	return len(p), nil
}

// writes status directly to the connection
func (w *Writer) WriteStatus(statusCode int) error {
	if w.writeStatus > StateWroteStatus {
		return ErrStatusAlreadyWritten
	}

	reason := http.StatusText(statusCode)
	if reason == "" {
		return errors.New("bad status code")
	}

	w.writeStatus = StateWroteStatus
	_, err := fmt.Fprintf(w.writer, "HTTP/1.1 %d %s\r\n", statusCode, reason)
	return err
}

// this implicitly removes transfer-encoding header
func (w *Writer) SetContentLength(contLenStr string) error {
	contLen, err := strconv.Atoi(contLenStr)
	if err != nil {
		return ErrInvalidContentLength
	}
	w.buf = make([]byte, 0, contLen)
	w.contentLen = contLen
	w.chunked = false
	w.headers.Set(ContentLength, contLenStr)
	w.headers.Del(TransferEncoding)
	return err
}

func (w *Writer) writeHeaders() error {
	if contLenStr, ok := w.headers.Get(ContentLength); ok {
		w.SetContentLength(contLenStr)
	} else {
		w.chunked = true
		w.headers.Set(TransferEncoding, "chunked")
	}

	hLines := []byte{}
	for key, vals := range w.headers {
		val := strings.Join(vals, ",")
		hLines = fmt.Appendf(hLines, "%s: %s\r\n", key, val)
	}
	hLines = fmt.Append(hLines, "\r\n")

	w.writeStatus = StateWroteHeader
	fmt.Println("about to write headers", string(hLines))
	_, err := w.writer.Write(hLines)
	return err
}

// Finish finalizes the response stream.
// - For chunked, writes the terminating 0-length chunk.
// - For Content-Length, returns error if body size not satisfied.
func (w *Writer) Finish() error {
	if err := w.upgradeWriteStatus(StateWroteHeader); err != nil {
		return err
	}
	if w.chunked {
		_, err := io.WriteString(w.writer, "0\r\n\r\n")
		return err
	}

	if w.buf != nil && len(w.buf) != w.contentLen {
		return io.ErrShortWrite
	}
	return nil
}

func DefaultHeaders() headers.Headers {
	h := headers.NewHeaders()
	h.Set("Connection", "close")
	h.Set("Content-Type", "text/plain")
	return h
}

var (
	ErrStatusAlreadyWritten       = errors.New("status already written")
	ErrWriteMoreThanContentLength = errors.New("attempting to write more than content lenght")
	ErrInvalidContentLength       = errors.New("invalid content length")
)
