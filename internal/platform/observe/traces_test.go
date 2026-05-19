package observe

import (
	"testing"
	"time"
)

func TestTraceContext_StartAndFinish(t *testing.T) {
	tc := NewTraceContext()
	span := tc.StartSpan("test.op", "")
	time.Sleep(1 * time.Millisecond)
	tc.FinishSpan(span)

	if tc.SpanCount() != 1 {
		t.Errorf("expected 1 span, got %d", tc.SpanCount())
	}
	spans := tc.Spans()
	if spans[0].Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestTraceContext_ParentChild(t *testing.T) {
	tc := NewTraceContext()
	parent := tc.StartSpan("parent", "")
	tc.FinishSpan(parent)

	child := tc.StartSpan("child", parent.SpanID)
	tc.FinishSpan(child)

	spans := tc.Spans()
	if spans[1].ParentID != spans[0].SpanID {
		t.Error("child should reference parent span ID")
	}
}

func TestTraceContext_FindByOperation(t *testing.T) {
	tc := NewTraceContext()
	s1 := tc.StartSpan("http.request", "")
	tc.FinishSpan(s1)
	s2 := tc.StartSpan("db.query", "")
	tc.FinishSpan(s2)
	s3 := tc.StartSpan("http.request", "")
	tc.FinishSpan(s3)

	found := tc.FindByOperation("http.request")
	if len(found) != 2 {
		t.Errorf("expected 2 http.request spans, got %d", len(found))
	}
}

func TestTraceContext_UniqueIDs(t *testing.T) {
	tc := NewTraceContext()
	s1 := tc.StartSpan("a", "")
	s2 := tc.StartSpan("b", "")

	if s1.SpanID == s2.SpanID {
		t.Error("span IDs should be unique")
	}
}

func TestTraceContext_SpanCount(t *testing.T) {
	tc := NewTraceContext()
	for i := 0; i < 5; i++ {
		s := tc.StartSpan("op", "")
		tc.FinishSpan(s)
	}
	if tc.SpanCount() != 5 {
		t.Errorf("expected 5, got %d", tc.SpanCount())
	}
}
