package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadersParser(t *testing.T) {
	headers := NewHeaders()
	data := []byte("Host: localhost:42069\r\nFooFoo: Barbar\r\n\r\n")
	n, done, err := headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:42069", headers.GetTest("host"))
	assert.Equal(t, "Barbar", headers.GetTest("FooFoo"))
	assert.Equal(t, 41, n)
	assert.True(t, done)

	// Test: Invalid spacing header
	headers = NewHeaders()
	data = []byte("       Host : localhost:42069       \r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)

	// Test: multivalue headers
	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\nFooFoo: Barbar\r\nFooFoo: Barbar2\r\n\r\n")
	_, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "localhost:42069", headers.GetTest("host"))
	assert.Equal(t, "Barbar,Barbar2", headers.GetTest("FooFoo"))
	assert.True(t, done)
}
