package protocol

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func strippedLines(t *testing.T, reader io.Reader) []string {
	t.Helper()
	var lines []string
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		// Skip the empty lines that separate SSE events; only data and
		// comment lines carry information here.
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stripped stream: %v", err)
	}
	return lines
}

func TestNullStripStreamReaderStripsFreeTierPlaceholders(t *testing.T) {
	stream := "data: {\"choices\":[{\"index\":0,\"finish_reason\":null,\"logprobs\":null,\"delta\":{\"role\":null,\"content\":null,\"reasoning_content\":null,\"tool_calls\":null}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"finish_reason\":null,\"delta\":{\"role\":\"assistant\",\"content\":\"OK\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\",\"delta\":{},\"usage\":{\"prompt_tokens\":4678,\"completion_tokens\":16}}]}\n\n" +
		"data: [DONE]\n\n"
	reader := NullStripStreamReader(strings.NewReader(stream), Chat)
	lines := strippedLines(t, reader)

	if len(lines) != 4 {
		t.Fatalf("stripped stream has %d lines, want 4", len(lines))
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[0], "data: ")), &first); err != nil {
		t.Fatalf("first frame is not JSON: %v", err)
	}
	choice := first["choices"].([]any)[0].(map[string]any)
	if _, has := choice["finish_reason"]; has {
		t.Fatalf("finish_reason null survived: %v", choice)
	}
	if _, has := choice["logprobs"]; has {
		t.Fatalf("logprobs null survived: %v", choice)
	}
	delta := choice["delta"].(map[string]any)
	for _, key := range []string{"role", "content", "reasoning_content", "tool_calls"} {
		if _, has := delta[key]; has {
			t.Fatalf("delta.%s null survived: %v", key, delta)
		}
	}

	if !strings.Contains(lines[1], "\"content\":\"OK\"") {
		t.Fatalf("content delta lost: %s", lines[1])
	}
	if !strings.Contains(lines[2], "\"finish_reason\":\"stop\"") || !strings.Contains(lines[2], "\"prompt_tokens\":4678") {
		t.Fatalf("final frame changed: %s", lines[2])
	}
	if lines[3] != "data: [DONE]" {
		t.Fatalf("terminator line changed: %s", lines[3])
	}
}

func TestNullStripStreamReaderKeepsCleanFramesByteForByte(t *testing.T) {
	clean := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hi\"}}]}\n\n"
	reader := NullStripStreamReader(strings.NewReader(clean), Chat)
	lines := strippedLines(t, reader)
	if len(lines) != 1 || lines[0] != strings.TrimRight(clean, "\n") {
		t.Fatalf("clean frame was re-encoded: %q", lines[0])
	}
}

func TestNullStripStreamReaderPassesNonDataLinesThrough(t *testing.T) {
	stream := ": keepalive\n\n" +
		"event: ping\n\n" +
		"data: not-json\n\n" +
		"data: {\"usage\":null}\n\n"
	reader := NullStripStreamReader(strings.NewReader(stream), Chat)
	lines := strippedLines(t, reader)
	if len(lines) != 4 {
		t.Fatalf("stream has %d lines, want 4", len(lines))
	}
	if lines[0] != ": keepalive" || lines[2] != "data: not-json" {
		t.Fatalf("non-data lines changed: %q / %q", lines[0], lines[2])
	}
	if strings.Contains(lines[3], "null") {
		t.Fatalf("usage null survived: %s", lines[3])
	}
}

func TestNullStripStreamReaderKeepsNullArrayElementsAndNumbers(t *testing.T) {
	stream := "data: {\"choices\":[{\"index\":12345678901234,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":null,\"type\":null,\"function\":{\"name\":\"webfetch\"}}]}},null]}\n\n"
	reader := NullStripStreamReader(strings.NewReader(stream), Chat)
	lines := strippedLines(t, reader)
	if !strings.Contains(lines[0], "\"name\":\"webfetch\"") {
		t.Fatalf("tool name lost: %s", lines[0])
	}
	if strings.Contains(lines[0], "\"id\":null") || strings.Contains(lines[0], "\"type\":null") {
		t.Fatalf("tool call nulls survived: %s", lines[0])
	}
	if !strings.Contains(lines[0], "12345678901234") {
		t.Fatalf("large number lost precision: %s", lines[0])
	}
	if !strings.Contains(lines[0], ",null]") {
		t.Fatalf("null array element was dropped: %s", lines[0])
	}
}

func TestNullStripStreamReaderIgnoresNonChatProtocols(t *testing.T) {
	stream := "data: {\"usage\":null}\n\n"
	reader := NullStripStreamReader(strings.NewReader(stream), Anthropic)
	lines := strippedLines(t, reader)
	if lines[0] != strings.TrimRight(stream, "\n") {
		t.Fatalf("anthropic frame was modified: %q", lines[0])
	}
}

func TestStripNullValuesDirect(t *testing.T) {
	out := StripNullValues([]byte(`{"choices":[{"index":12345678901234,"delta":{"tool_calls":[{"index":0,"id":null,"type":null,"function":{"name":"webfetch"}}]}},null]}`))
	if out == nil {
		t.Fatal("StripNullValues returned nil for a frame with nulls")
	}
	if strings.Contains(string(out), `"id":null`) || strings.Contains(string(out), `"type":null`) {
		t.Fatalf("object nulls survived: %s", out)
	}
	if !strings.Contains(string(out), ",null]") {
		t.Fatalf("null array element was dropped: %s", out)
	}
	if !strings.Contains(string(out), "12345678901234") {
		t.Fatalf("large number lost precision: %s", out)
	}
}
