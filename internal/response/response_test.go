package response

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parsed struct {
	statusLine string
	headers    map[string]string
	body       string
}

func parseHTTP(raw []byte) parsed {
	parts := strings.SplitN(string(raw), "\r\n\r\n", 2)
	p := parsed{headers: map[string]string{}}
	if len(parts) == 0 || parts[0] == "" {
		return p
	}
	headerBlock := parts[0]
	lines := strings.Split(headerBlock, "\r\n")
	if len(lines) > 0 {
		p.statusLine = lines[0]
		for _, ln := range lines[1:] {
			if ln == "" {
				continue
			}
			kv := strings.SplitN(ln, ":", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			v := strings.TrimSpace(kv[1])
			p.headers[k] = v
		}
	}
	if len(parts) == 2 {
		p.body = parts[1]
	}
	return p
}

func Test_DefaultChunkedResponse(t *testing.T) {
	var buf bytes.Buffer
	w := NewResponseWriter(&buf)

	_, err := w.Write([]byte("Hello"))
	require.NoError(t, err)
	require.NoError(t, w.Finish())

	p := parseHTTP(buf.Bytes())
	assert.Equal(t, "HTTP/1.1 200 OK", p.statusLine)
	assert.Equal(t, "close", p.headers["connection"])
	assert.Equal(t, "text/plain", p.headers["content-type"])
	assert.Equal(t, "chunked", p.headers["transfer-encoding"])
	_, hasCL := p.headers["content-length"]
	assert.False(t, hasCL)
	assert.Equal(t, "5\r\nHello\r\n0\r\n\r\n", p.body)
}

func Test_ContentLengthResponse(t *testing.T) {
	var buf bytes.Buffer
	w := NewResponseWriter(&buf)
	body := "abc123"
	cl := strconv.Itoa(len(body))
	w.Headers().Set(ContentLength, cl)

	_, err := w.Write([]byte(body))
	require.NoError(t, err)
	require.NoError(t, w.Finish())

	p := parseHTTP(buf.Bytes())
	assert.Equal(t, "HTTP/1.1 200 OK", p.statusLine)
	assert.Equal(t, "close", p.headers["connection"])
	assert.Equal(t, "text/plain", p.headers["content-type"])
	assert.Equal(t, cl, p.headers["content-length"])
	_, hasTE := p.headers["transfer-encoding"]
	assert.False(t, hasTE)
	assert.Equal(t, body, p.body)
}

func Test_StatusOverride(t *testing.T) {
	var buf bytes.Buffer
	w := NewResponseWriter(&buf)
	require.NoError(t, w.WriteStatus(400))

	_, err := w.Write([]byte("oops"))
	require.NoError(t, err)
	require.NoError(t, w.Finish())

	p := parseHTTP(buf.Bytes())
	assert.Equal(t, "HTTP/1.1 400 Bad Request", p.statusLine)
}

func Test_FinishShortWrite(t *testing.T) {
	var buf bytes.Buffer
	w := NewResponseWriter(&buf)
	w.Headers().Set(ContentLength, "5")
	_, err := w.Write([]byte("hi"))
	require.NoError(t, err)
	err = w.Finish()
	require.Error(t, err)
	assert.Equal(t, io.ErrShortWrite, err)

	p := parseHTTP(buf.Bytes())
	assert.Equal(t, "5", p.headers["content-length"])
	assert.Equal(t, "hi", p.body)
}

func Test_WriteMoreThanContentLength(t *testing.T) {
	var buf bytes.Buffer
	w := NewResponseWriter(&buf)
	w.Headers().Set(ContentLength, "3")
	_, err := w.Write([]byte("abcd"))
	require.Error(t, err)
	assert.Equal(t, ErrWriteMoreThanContentLength, err)
}
