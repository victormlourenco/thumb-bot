package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitReadMore_ShortText(t *testing.T) {
	preview, rest := splitReadMore("Hello world")
	if preview != "Hello world" {
		t.Fatalf("preview=%q", preview)
	}
	if rest != "" {
		t.Fatalf("rest=%q, want empty", rest)
	}
}

func TestSplitReadMore_LongByRunes(t *testing.T) {
	body := strings.Repeat("word ", 80) // 400 runes
	preview, rest := splitReadMore(body)
	if rest == "" {
		t.Fatal("expected rest for long text")
	}
	if utf8.RuneCountInString(preview) > previewMaxRunes {
		t.Fatalf("preview has %d runes, max %d", utf8.RuneCountInString(preview), previewMaxRunes)
	}
	if !strings.HasPrefix(strings.TrimSpace(body), preview) {
		t.Fatalf("preview is not a prefix of body: %q", preview)
	}
}

func TestSplitReadMore_LongByLines(t *testing.T) {
	lines := make([]string, 8)
	for i := range lines {
		lines[i] = "line of caption text"
	}
	body := strings.Join(lines, "\n")
	preview, rest := splitReadMore(body)
	if rest == "" {
		t.Fatal("expected rest for many lines")
	}
	if strings.Count(preview, "\n")+1 > previewMaxLines {
		t.Fatalf("preview has too many lines: %q", preview)
	}
}

func TestSplitReadMore_TinyRemainderKeptIntact(t *testing.T) {
	previewLimit := strings.Repeat("a", previewMaxRunes)
	body := previewLimit + " short"
	preview, rest := splitReadMore(body)
	if rest != "" {
		t.Fatalf("tiny remainder should not collapse, rest=%q", rest)
	}
	if preview != body {
		t.Fatalf("preview=%q, want full body", preview)
	}
}

func TestNeedsReadMore(t *testing.T) {
	if needsReadMore("short caption") {
		t.Fatal("short caption should not need read more")
	}
	if !needsReadMore(strings.Repeat("long text ", 50)) {
		t.Fatal("long caption should need read more")
	}
}

func TestWriteHeaderAndCollapsibleBody_Short(t *testing.T) {
	var sb strings.Builder
	writeHeaderAndCollapsibleBody(&sb, "<b>Author</b>", "Hello")
	got := sb.String()
	if strings.Contains(got, "<details>") {
		t.Fatalf("short body should not collapse: %s", got)
	}
	if !strings.Contains(got, "<b>Author</b>") || !strings.Contains(got, "Hello") {
		t.Fatalf("missing header or body: %s", got)
	}
}

func TestWriteHeaderAndCollapsibleBody_Long(t *testing.T) {
	var sb strings.Builder
	writeHeaderAndCollapsibleBody(&sb, "<b>Author</b>", strings.Repeat("lots of caption text ", 40))
	got := sb.String()
	if !strings.Contains(got, "<details><summary>Read more...</summary>") {
		t.Fatalf("missing details summary: %s", got)
	}
	if !strings.Contains(got, "</details>") {
		t.Fatalf("unclosed details: %s", got)
	}
	if strings.Count(got, "<b>Author</b>") != 1 {
		t.Fatalf("author should stay outside the collapse: %s", got)
	}
}

func TestFormatCaptionWithReadMore_Long(t *testing.T) {
	got := formatCaptionWithReadMore("<b>Author</b>", strings.Repeat("lots of caption text ", 40))
	if !strings.Contains(got, "<blockquote expandable>") {
		t.Fatalf("fallback caption should use expandable quote: %s", got)
	}
	if !strings.HasPrefix(got, "<b>Author</b>:\n") {
		t.Fatalf("header should stay visible: %s", got)
	}
}
