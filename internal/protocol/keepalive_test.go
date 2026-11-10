package protocol

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// readWithin reads once in a goroutine so a blocked source cannot hang the
// test. It returns the chunk, how long the read took, and whether it finished.
func readWithin(reader io.Reader, wait time.Duration) (string, time.Duration, bool) {
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	started := time.Now()
	go func() {
		buffer := make([]byte, 64)
		n, err := reader.Read(buffer)
		done <- result{data: buffer[:n], err: err}
	}()
	select {
	case got := <-done:
		return string(got.data), time.Since(started), true
	case <-time.After(wait):
		return "", time.Since(started), false
	}
}

// TestKeepAliveReaderIdleLatency quantifies the change: with the same 400ms
// upstream pause, the raw stream delivers nothing until the pause ends while
// the wrapped stream delivers a comment after one 50ms interval.
func TestKeepAliveReaderIdleLatency(t *testing.T) {
	const pause = 400 * time.Millisecond
	const interval = 50 * time.Millisecond

	t.Run("baseline raw stream", func(t *testing.T) {
		reader, writer := io.Pipe()
		defer reader.Close()
		go func() {
			_, _ = writer.Write([]byte("data: first\n\n"))
			time.Sleep(pause)
			_, _ = writer.Write([]byte("data: second\n\n"))
		}()
		if got, _, ok := readWithin(reader, time.Second); !ok || got != "data: first\n\n" {
			t.Fatalf("first event = %q, ok=%v", got, ok)
		}
		got, elapsed, ok := readWithin(reader, 2*time.Second)
		if !ok {
			t.Fatal("raw stream did not deliver the second event")
		}
		if got != "data: second\n\n" {
			t.Fatalf("second event = %q", got)
		}
		if elapsed < 300*time.Millisecond {
			t.Fatalf("raw stream returned too early (%v); pause was not simulated", elapsed)
		}
		t.Logf("baseline first byte after idle: %v (event only)", elapsed)
	})

	t.Run("wrapped keepalive stream", func(t *testing.T) {
		reader, writer := io.Pipe()
		defer reader.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wrapped := NewKeepAliveReader(ctx, reader, interval)
		go func() {
			_, _ = writer.Write([]byte("data: first\n\n"))
			time.Sleep(pause)
			_, _ = writer.Write([]byte("data: second\n\n"))
		}()
		if got, _, ok := readWithin(wrapped, time.Second); !ok || got != "data: first\n\n" {
			t.Fatalf("first event = %q, ok=%v", got, ok)
		}
		got, elapsed, ok := readWithin(wrapped, 2*time.Second)
		if !ok {
			t.Fatal("wrapped stream did not deliver keepalive")
		}
		if got != streamKeepAliveFrame {
			t.Fatalf("idle chunk = %q, want %q", got, streamKeepAliveFrame)
		}
		if elapsed < interval/2 || elapsed > pause/2 {
			t.Fatalf("keepalive latency = %v, want about %v", elapsed, interval)
		}
		t.Logf("candidate first byte after idle: %v (keepalive)", elapsed)
		keepAlives := 1
		for {
			got, _, ok = readWithin(wrapped, 2*time.Second)
			if !ok {
				t.Fatal("wrapped stream did not deliver the second event")
			}
			if got == streamKeepAliveFrame {
				keepAlives++
				continue
			}
			break
		}
		if got != "data: second\n\n" {
			t.Fatalf("second event = %q", got)
		}
		if keepAlives < 2 {
			t.Fatalf("keepalives observed = %d, want repeated frames while idle", keepAlives)
		}
		t.Logf("keepalives observed during %v idle: %d", pause, keepAlives)
	})
}

// TestKeepAliveReaderNeverSplitsAnEvent writes half an event, waits four
// intervals, and proves the comment is withheld until the blank line arrives.
func TestKeepAliveReaderNeverSplitsAnEvent(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wrapped := NewKeepAliveReader(ctx, reader, 50*time.Millisecond)

	go func() {
		_, _ = writer.Write([]byte("data: par"))
	}()
	first, _, ok := readWithin(wrapped, time.Second)
	if !ok || first != "data: par" {
		t.Fatalf("partial chunk = %q, ok=%v", first, ok)
	}

	done := make(chan string, 1)
	go func() {
		buffer := make([]byte, 64)
		n, _ := wrapped.Read(buffer)
		done <- string(buffer[:n])
	}()
	select {
	case got := <-done:
		t.Fatalf("reader returned %q while the event had no blank line", got)
	case <-time.After(200 * time.Millisecond):
	}

	if _, err := writer.Write([]byte("tial\n\n")); err != nil {
		t.Fatalf("write event tail: %v", err)
	}
	select {
	case rest := <-done:
		if rest != "tial\n\n" {
			t.Fatalf("event tail = %q", rest)
		}
	case <-time.After(time.Second):
		t.Fatal("reader did not deliver the completed event")
	}
}

// TestKeepAliveReaderDisabled keeps the option honest: zero must not wrap.
func TestKeepAliveReaderDisabled(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	if got := NewKeepAliveReader(context.Background(), reader, 0); got != reader {
		t.Fatal("zero interval must return the source reader unchanged")
	}
	_ = writer.Close()
}

// TestForwardStreamFlushesKeepAlive proves the comment reaches a real client
// during an upstream pause, not merely that the wrapper returns it to io.Copy.
func TestForwardStreamFlushesKeepAlive(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		wrapped := NewKeepAliveReader(r.Context(), reader, 50*time.Millisecond)
		if _, _, err := ForwardStream(r.Context(), w, wrapped, Chat, "keepalive-test"); err != nil {
			t.Logf("ForwardStream: %v", err)
		}
	}))
	defer server.Close()

	go func() {
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"a\"},\"finish_reason\":null}]}\n\n"))
		time.Sleep(400 * time.Millisecond)
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"b\"},\"finish_reason\":null}]}\n\n"))
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
		_ = writer.Close()
	}()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	lines := bufio.NewReader(resp.Body)
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			t.Fatalf("read first event: %v", err)
		}
		if line == "\n" {
			break
		}
	}

	started := time.Now()
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			t.Fatalf("read keepalive: %v", err)
		}
		if strings.HasPrefix(line, ": keepalive") {
			break
		}
		if strings.HasPrefix(line, "data:") {
			t.Fatalf("second event arrived before keepalive: %q", line)
		}
	}
	elapsed := time.Since(started)
	if elapsed > 250*time.Millisecond {
		t.Fatalf("keepalive arrived after %v, want about 50ms", elapsed)
	}
	t.Logf("ForwardStream delivered keepalive after %v", elapsed)
}
