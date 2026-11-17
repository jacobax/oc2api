package identity

import (
	"regexp"
	"testing"
)

var (
	b3Pattern          = regexp.MustCompile(`^[0-9a-f]{32}-[0-9a-f]{16}-1-[0-9a-f]{16}$`)
	traceparentPattern = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-01$`)
)

func TestTraceHeadersShape(t *testing.T) {
	b3, traceparent := TraceHeaders("ses_000000000000example", "req_one")
	if !b3Pattern.MatchString(b3) {
		t.Fatalf("b3 %q does not match the B3 single-header layout", b3)
	}
	if !traceparentPattern.MatchString(traceparent) {
		t.Fatalf("traceparent %q does not match the W3C layout", traceparent)
	}
	if b3[:32] != traceparent[3:35] || b3[33:49] != traceparent[36:52] {
		t.Fatalf("b3 %q and traceparent %q disagree on trace or span", b3, traceparent)
	}
}

func TestTraceHeadersTraceStablePerSessionSpanPerRequest(t *testing.T) {
	session := "ses_000000000000example"
	first, _ := TraceHeaders(session, "req_one")
	second, _ := TraceHeaders(session, "req_two")
	if first[:32] != second[:32] {
		t.Fatalf("trace changed across requests of one session: %s vs %s", first[:32], second[:32])
	}
	if first[33:49] == second[33:49] {
		t.Fatalf("span did not rotate across requests: %s", first[33:49])
	}
	other, _ := TraceHeaders("ses_ffffffffffffexample", "req_one")
	if other[:32] == first[:32] {
		t.Fatalf("two sessions share one trace: %s", other[:32])
	}
}

func TestTraceHeadersEmptySessionFallsBack(t *testing.T) {
	b3, traceparent := TraceHeaders("", "req_only")
	if !b3Pattern.MatchString(b3) || !traceparentPattern.MatchString(traceparent) {
		t.Fatalf("empty session produced malformed headers: %q / %q", b3, traceparent)
	}
}
