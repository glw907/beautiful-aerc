package logctx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWithOpID_RoundTrip(t *testing.T) {
	ctx := WithOpID(context.Background(), "42")
	id, ok := ctx.Value(opIDKey{}).(string)
	if !ok || id != "42" {
		t.Fatalf("got %q ok=%v, want 42 true", id, ok)
	}
}

func TestHandler_InjectsOpID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := Handler{inner}
	log := slog.New(h)

	ctx := WithOpID(context.Background(), "99")
	log.DebugContext(ctx, "test event")

	if !strings.Contains(buf.String(), "op_id=99") {
		t.Errorf("op_id not in output: %s", buf.String())
	}
}

func TestHandler_NoOpID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := Handler{inner}
	log := slog.New(h)

	log.DebugContext(context.Background(), "no op")

	if strings.Contains(buf.String(), "op_id") {
		t.Errorf("unexpected op_id in output: %s", buf.String())
	}
}

func TestHandler_WithAttrs_PreservesInjection(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	log := slog.New(Handler{inner}).With("component", "cache")

	ctx := WithOpID(context.Background(), "7")
	log.DebugContext(ctx, "cache event")

	out := buf.String()
	if !strings.Contains(out, "op_id=7") {
		t.Errorf("op_id missing after With: %s", out)
	}
	if !strings.Contains(out, "component=cache") {
		t.Errorf("component missing after With: %s", out)
	}
}

func TestWireWriter_EmitsLines(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(inner))
	t.Cleanup(func() { slog.SetDefault(prev) })

	w := WireWriter{Component: "imap"}
	_, _ = w.Write([]byte("A001 LOGIN user pass\nA002 SELECT INBOX\n"))

	out := buf.String()
	if !strings.Contains(out, "A001 LOGIN user pass") {
		t.Errorf("first line missing: %s", out)
	}
	if !strings.Contains(out, "A002 SELECT INBOX") {
		t.Errorf("second line missing: %s", out)
	}
	if !strings.Contains(out, "component=imap") {
		t.Errorf("component missing: %s", out)
	}
}

func TestWireWriter_SkipsEmptyLines(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(inner))
	t.Cleanup(func() { slog.SetDefault(prev) })

	w := WireWriter{Component: "smtp"}
	_, _ = w.Write([]byte("\n\nfoo\n"))

	lines := strings.Count(buf.String(), "\n")
	if lines != 1 {
		t.Errorf("want 1 line, got %d: %s", lines, buf.String())
	}
}
