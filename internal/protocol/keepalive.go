package protocol

import (
	"bytes"
	"context"
	"io"
	"time"
)

// streamKeepAliveFrame is an SSE comment. Clients and SSE parsers ignore it,
// while a proxy that closes idle connections sees traffic and keeps the
// response open.
const streamKeepAliveFrame = ": keepalive\n\n"

// keepAliveRead is one chunk read from the upstream body.
type keepAliveRead struct {
	data []byte
	err  error
}

// keepAliveReader emits an SSE comment when the upstream body has been idle at
// an event boundary for longer than interval. It reads the source in a
// goroutine because io.Reader has no timeout, and only injects a comment after
// a blank line: inserting one in the middle of a data line would corrupt that
// event.
type keepAliveReader struct {
	ctx      context.Context
	source   io.Reader
	interval time.Duration
	results  chan keepAliveRead

	pending  []byte
	err      error
	tail     []byte
	boundary bool
}

// NewKeepAliveReader wraps reader so an idle SSE stream still produces traffic.
// A non-positive interval returns reader unchanged. The returned reader must be
// closed through the underlying body so the pump goroutine can exit.
func NewKeepAliveReader(ctx context.Context, reader io.Reader, interval time.Duration) io.Reader {
	if reader == nil || interval <= 0 {
		return reader
	}
	if ctx == nil {
		ctx = context.Background()
	}
	keepAlive := &keepAliveReader{
		ctx:      ctx,
		source:   reader,
		interval: interval,
		results:  make(chan keepAliveRead, 1),
		boundary: true,
	}
	go keepAlive.pump()
	return keepAlive
}

func (reader *keepAliveReader) pump() {
	defer close(reader.results)
	buffer := make([]byte, 32<<10)
	for {
		n, err := reader.source.Read(buffer)
		if n > 0 {
			data := append([]byte(nil), buffer[:n]...)
			select {
			case reader.results <- keepAliveRead{data: data}:
			case <-reader.ctx.Done():
				return
			}
		}
		if err != nil {
			select {
			case reader.results <- keepAliveRead{err: err}:
			case <-reader.ctx.Done():
			}
			return
		}
	}
}

func (reader *keepAliveReader) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	for {
		if len(reader.pending) > 0 {
			n := copy(data, reader.pending)
			reader.pending = reader.pending[n:]
			return n, nil
		}
		if reader.err != nil {
			err := reader.err
			reader.err = nil
			return 0, err
		}
		timer := time.NewTimer(reader.interval)
		select {
		case result, ok := <-reader.results:
			timer.Stop()
			if !ok {
				reader.err = io.EOF
				continue
			}
			if len(result.data) > 0 {
				reader.observe(result.data)
				reader.pending = result.data
			}
			if result.err != nil {
				reader.err = result.err
			}
		case <-timer.C:
			if reader.boundary {
				frame := []byte(streamKeepAliveFrame)
				if len(data) >= len(frame) {
					return copy(data, frame), nil
				}
				reader.pending = frame
			}
		case <-reader.ctx.Done():
			timer.Stop()
			return 0, reader.ctx.Err()
		}
	}
}

// observe updates whether the bytes seen so far end at an SSE event boundary.
// A comment is safe only after a blank line, so the last few bytes are enough
// to decide.
func (reader *keepAliveReader) observe(data []byte) {
	reader.tail = append(reader.tail, data...)
	if len(reader.tail) > 4 {
		reader.tail = reader.tail[len(reader.tail)-4:]
	}
	reader.boundary = bytes.HasSuffix(reader.tail, []byte("\n\n")) ||
		bytes.HasSuffix(reader.tail, []byte("\r\n\r\n"))
}
