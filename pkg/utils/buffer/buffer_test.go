// buffer_test.go
package buffer

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestBuffer_Bytes(t *testing.T) {
	buf := NewBuffer([]byte("hello"))
	if got := string(buf.Bytes()); got != "hello" {
		t.Errorf("Bytes() = %q, want %q", got, "hello")
	}

	buf.Truncate(3)
	if got := string(buf.Bytes()); got != "hel" {
		t.Errorf("After Truncate(3), Bytes() = %q, want %q", got, "hel")
	}
}

func TestBuffer_String(t *testing.T) {
	buf := NewBuffer([]byte("world"))
	if got := buf.String(); got != "world" {
		t.Errorf("String() = %q, want %q", got, "world")
	}

	var nilBuf *Buffer
	if got := nilBuf.String(); got != "<nil>" {
		t.Errorf("(*Buffer)(nil).String() = %q, want \"<nil>\"", got)
	}
}

func TestBuffer_Len(t *testing.T) {
	buf := NewBuffer([]byte("test"))
	if buf.Len() != 4 {
		t.Errorf("Len() = %d, want 4", buf.Len())
	}

	buf.Truncate(2)
	if buf.Len() != 2 {
		t.Errorf("After Truncate(2), Len() = %d, want 2", buf.Len())
	}

	buf.Reset()
	if buf.Len() != 0 {
		t.Errorf("After Reset(), Len() = %d, want 0", buf.Len())
	}
}

func TestBuffer_Truncate(t *testing.T) {
	buf := NewBuffer([]byte("abcdef"))
	buf.Truncate(3)
	if string(buf.Bytes()) != "abc" {
		t.Errorf("After Truncate(3), got %q, want \"abc\"", string(buf.Bytes()))
	}

	// Truncate(0) should reset
	buf.Truncate(0)
	if buf.Len() != 0 || buf.off != 0 || len(buf.buf) != 0 {
		t.Errorf("Truncate(0) should reset buffer")
	}

	// Test panic on invalid n
	defer func() {
		if r := recover(); r == nil {
			t.Error("Truncate(-1) should panic")
		}
	}()
	buf = NewBuffer([]byte("x"))
	buf.Truncate(-1)
}

func TestBuffer_Reset(t *testing.T) {
	buf := NewBuffer([]byte("data"))
	buf.Reset()
	if buf.Len() != 0 || buf.off != 0 || len(buf.buf) != 0 {
		t.Errorf("Reset() did not clear buffer properly")
	}
}

func TestBuffer_Alloc(t *testing.T) {
	buf := new(Buffer)
	s := buf.Alloc(5)
	if len(s) != 5 {
		t.Fatalf("Alloc(5) returned slice of len %d", len(s))
	}
	// Write to allocated space
	copy(s, "hello")
	if buf.String() != "hello" {
		t.Errorf("After Alloc and copy, buffer = %q, want \"hello\"", buf.String())
	}

	// Test negative panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Alloc(-1) should panic")
		}
	}()
	buf.Alloc(-1)
}

func TestBuffer_Grow(t *testing.T) {
	buf := new(Buffer)
	buf.Grow(100)
	// Grow doesn't change Len(), only capacity
	if buf.Len() != 0 {
		t.Errorf("Grow should not change Len(), got %d", buf.Len())
	}

	// Now write more than initial cap
	data := make([]byte, 200)
	for i := range data {
		data[i] = byte('a' + i%26)
	}
	n, err := buf.Write(data)
	if err != nil || n != len(data) {
		t.Errorf("Write after Grow failed: n=%d, err=%v", n, err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Error("Written data does not match")
	}

	// Test negative panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Grow(-1) should panic")
		}
	}()
	buf.Grow(-1)
}

func TestBuffer_Write(t *testing.T) {
	buf := new(Buffer)
	input := []byte("hello world")
	n, err := buf.Write(input)
	if err != nil || n != len(input) {
		t.Fatalf("Write failed: n=%d, err=%v", n, err)
	}
	if !bytes.Equal(buf.Bytes(), input) {
		t.Errorf("Buffer content = %q, want %q", buf.Bytes(), input)
	}
}

func TestBuffer_WriteByte(t *testing.T) {
	buf := new(Buffer)
	err := buf.WriteByte('x')
	if err != nil {
		t.Fatalf("WriteByte failed: %v", err)
	}
	if buf.String() != "x" {
		t.Errorf("After WriteByte('x'), got %q", buf.String())
	}
}

func TestBuffer_Read(t *testing.T) {
	buf := NewBuffer([]byte("12345"))
	p := make([]byte, 3)
	n, err := buf.Read(p)
	if err != nil || n != 3 {
		t.Fatalf("Read failed: n=%d, err=%v", n, err)
	}
	if string(p[:n]) != "123" {
		t.Errorf("Read data = %q, want \"123\"", string(p[:n]))
	}

	// Read remaining
	n, err = buf.Read(p)
	if err != nil || n != 2 {
		t.Fatalf("Second Read failed: n=%d, err=%v", n, err)
	}
	if string(p[:n]) != "45" {
		t.Errorf("Second read = %q, want \"45\"", string(p[:n]))
	}

	// Read on empty buffer
	n, err = buf.Read(p)
	if err != io.EOF || n != 0 {
		t.Errorf("Read on empty: n=%d, err=%v, want n=0, err=EOF", n, err)
	}

	// Read with zero-length slice
	n, err = buf.Read([]byte{})
	if err != nil || n != 0 {
		t.Errorf("Read([]byte{}) = %d, %v; want 0, nil", n, err)
	}
}

func TestBuffer_ReadByte(t *testing.T) {
	buf := NewBuffer([]byte("AB"))
	b, err := buf.ReadByte()
	if err != nil || b != 'A' {
		t.Fatalf("ReadByte 1: b=%c, err=%v", b, err)
	}
	b, err = buf.ReadByte()
	if err != nil || b != 'B' {
		t.Fatalf("ReadByte 2: b=%c, err=%v", b, err)
	}
	_, err = buf.ReadByte()
	if err != io.EOF {
		t.Errorf("Third ReadByte should return EOF, got %v", err)
	}
}

func TestBuffer_Next(t *testing.T) {
	buf := NewBuffer([]byte("abcdef"))
	s := buf.Next(3)
	if string(s) != "abc" {
		t.Errorf("Next(3) = %q, want \"abc\"", s)
	}
	if buf.Len() != 3 {
		t.Errorf("After Next(3), Len() = %d, want 3", buf.Len())
	}

	s = buf.Next(10) // more than available
	if string(s) != "def" {
		t.Errorf("Next(10) = %q, want \"def\"", s)
	}
	if buf.Len() != 0 {
		t.Errorf("After final Next, Len() = %d, want 0", buf.Len())
	}
}

func TestBuffer_ReadBytes(t *testing.T) {
	buf := NewBuffer([]byte("hello\nworld\n"))
	line, err := buf.ReadBytes('\n')
	if err != nil || string(line) != "hello\n" {
		t.Errorf("First ReadBytes: %q, err=%v", line, err)
	}

	line, err = buf.ReadBytes('\n')
	if err != nil || string(line) != "world\n" {
		t.Errorf("Second ReadBytes: %q, err=%v", line, err)
	}

	line, err = buf.ReadBytes('\n')
	if err != io.EOF || string(line) != "" {
		t.Errorf("Final ReadBytes: line=%q, err=%v, want \"\", EOF", line, err)
	}

	// Test without delimiter
	buf = NewBuffer([]byte("no delimiter"))
	line, err = buf.ReadBytes('\n')
	if err != io.EOF || string(line) != "no delimiter" {
		t.Errorf("ReadBytes without delim: %q, err=%v", line, err)
	}
}

func TestBuffer_ReadFrom(t *testing.T) {
	buf := new(Buffer)
	r := bytes.NewReader([]byte("from reader"))
	n, err := buf.ReadFrom(r)
	if err != nil || n != 11 {
		t.Fatalf("ReadFrom failed: n=%d, err=%v", n, err)
	}
	if buf.String() != "from reader" {
		t.Errorf("Buffer = %q, want \"from reader\"", buf.String())
	}

	// Test error propagation
	errReader := &errorReader{err: errors.New("read error")}
	buf2 := new(Buffer)
	n, err = buf2.ReadFrom(errReader)
	if err == nil || n != 0 {
		t.Errorf("ReadFrom with error reader: n=%d, err=%v, want n=0, err!=nil", n, err)
	}
}

func TestBuffer_WriteTo(t *testing.T) {
	buf := NewBuffer([]byte("write to me"))
	var w bytes.Buffer
	n, err := buf.WriteTo(&w)
	if err != nil || n != 11 {
		t.Fatalf("WriteTo failed: n=%d, err=%v", n, err)
	}
	if w.String() != "write to me" {
		t.Errorf("Writer content = %q, want \"write to me\"", w.String())
	}
	if buf.Len() != 0 {
		t.Errorf("After WriteTo, buffer should be empty, Len()=%d", buf.Len())
	}

	// Test short write
	shortWriter := &shortWriter{}
	buf = NewBuffer([]byte("short"))
	n, err = buf.WriteTo(shortWriter)
	if err != io.ErrShortWrite {
		t.Errorf("Expected io.ErrShortWrite, got %v", err)
	}
}

// errorReader implements io.Reader and always returns an error.
type errorReader struct {
	err error
}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

// shortWriter implements io.Writer but writes fewer bytes than requested.
type shortWriter struct{}

func (sw *shortWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil // short write
}

func TestBuffer_ZeroValue(t *testing.T) {
	var buf Buffer // zero value
	if buf.Len() != 0 {
		t.Errorf("Zero Buffer.Len() = %d, want 0", buf.Len())
	}
	if buf.String() != "" {
		t.Errorf("Zero Buffer.String() = %q, want \"\"", buf.String())
	}

	// Should be usable
	buf.WriteString("ok")
	if buf.String() != "ok" {
		t.Errorf("After write, got %q", buf.String())
	}
}

// Helper method not in original, but useful for test
func (b *Buffer) WriteString(s string) (n int, err error) {
	return b.Write([]byte(s))
}
