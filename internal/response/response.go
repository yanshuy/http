package response

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/yanshuy/http/internal/headers"
)

const ContentLength = "Content-Length"
const TransferEncoding = "Transfer-Encoding"

type Response struct {
	headers    headers.Headers
	statusCode int
	contentLen int
	chunked    bool
}

func NewResponse(h headers.Headers) *Response {
	return &Response{
		headers:    h,
		statusCode: 200,
		chunked:    true,
	}
}

type writeState int

const (
	StateInitial writeState = iota
	StateWroteStatus
	StateWroteHeader
	StateWroteBody
	StateError
)

type Writer struct {
	*Response
	writer io.Writer
	writeState
	bytesWritten int
	lastError    error
}

func NewResponseWriter(w io.Writer) *Writer {
	h := DefaultHeaders()
	resp := NewResponse(h)
	return &Writer{
		Response:   resp,
		writer:     w,
		writeState: StateInitial,
	}
}

func (w *Writer) Headers() headers.Headers {
	return w.headers
}

func (w *Writer) upgradeWriteStatus(ws writeState) error {
	for w.writeState < ws {
		switch w.writeState {
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

func (w *Writer) setWriteError(err error) error {
	w.lastError = err
	w.writeState = StateError
	return err
}

// transfer encoding chucked is used when content-length is not set
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.lastError != nil {
		return 0, w.lastError
	}
	if err := w.upgradeWriteStatus(StateWroteHeader); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}

	if w.chunked {
		err := w.writeChunk(p)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	}

	if w.bytesWritten+len(p) > w.contentLen {
		w.setWriteError(ErrWriteMoreThanContentLength)
		return 0, ErrWriteMoreThanContentLength
	}

	n, err = w.writer.Write(p)
	if err != nil {
		w.setWriteError(err)
	}
	w.bytesWritten += n
	return n, err
}

func (w *Writer) writeChunk(p []byte) error {
	chunkHeader := fmt.Sprintf("%x\r\n", len(p))

	chunk := make([]byte, 0, len(chunkHeader)+len(p)+2)
	chunk = append(chunk, chunkHeader...)
	chunk = append(chunk, p...)
	chunk = append(chunk, '\r', '\n')

	_, err := w.writer.Write(chunk)
	if err != nil {
		return w.setWriteError(err)
	}
	return nil
}

// writes status directly to the connection
func (w *Writer) WriteStatus(statusCode int) error {
	if w.writeState > StateWroteStatus {
		return ErrStatusAlreadyWritten
	}

	reason := http.StatusText(statusCode)
	if reason == "" {
		return errors.New("bad status code")
	}

	_, err := fmt.Fprintf(w.writer, "HTTP/1.1 %d %s\r\n", statusCode, reason)
	if err != nil {
		return w.setWriteError(err)
	}
	w.statusCode = statusCode
	w.writeState = StateWroteStatus
	return nil
}

func (w *Writer) writeHeaders() error {
	if contLenStr, ok := w.headers.Get(ContentLength); ok {
		contLen, err := strconv.Atoi(contLenStr)
		if err != nil {
			return ErrInvalidContentLength
		}
		w.contentLen = contLen
		w.chunked = false
		w.headers.Del(TransferEncoding)
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

	// fmt.Println("About to write headers!\n", string(hLines))
	_, err := w.writer.Write(hLines)
	if err != nil {
		return w.setWriteError(err)
	}
	w.writeState = StateWroteHeader
	return nil
}

// Finish finalizes the response stream. returns any write error
// - For chunked, writes the terminating 0-length chunk.
// TODO: Trailers
func (w *Writer) Finish() error {
	if w.lastError != nil {
		return w.lastError
	}
	if w.writeState < StateWroteHeader {
		w.headers.Set(ContentLength, "0")
		if err := w.upgradeWriteStatus(StateWroteHeader); err != nil {
			return err
		}
	}

	if w.chunked {
		_, err := io.WriteString(w.writer, "0\r\n\r\n")
		return err
	}

	if w.bytesWritten != w.contentLen {
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
