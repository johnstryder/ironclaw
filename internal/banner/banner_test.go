package banner

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestStartup_WhenOptsNoDelay_ShouldPrintBannerAndVersionToWriter(t *testing.T) {
	var buf bytes.Buffer
	Startup("1.0.15", &StartupOpts{Writer: &buf, NoDelay: true})
	out := buf.String()
	if !strings.Contains(out, "agent framework") {
		t.Errorf("output should contain 'agent framework', got %q", out)
	}
	if !strings.Contains(out, "v1.0.15") {
		t.Errorf("output should contain 'v1.0.15', got %q", out)
	}
	if !strings.Contains(out, "@@@") {
		t.Errorf("output should contain banner art (@@@), got %q", out)
	}
}

func TestSplitLines_WhenEmptyString_ShouldReturnEmptySlice(t *testing.T) {
	got := splitLines("")
	if len(got) != 0 {
		t.Errorf("splitLines(%q): want 0 lines, got %d", "", len(got))
	}
}

func TestSplitLines_WhenSingleLine_ShouldReturnOneElement(t *testing.T) {
	got := splitLines("hello")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("splitLines('hello'): want [hello], got %q", got)
	}
}

func TestSplitLines_WhenMultipleLines_ShouldSplitByNewline(t *testing.T) {
	got := splitLines("a\nb\nc")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("splitLines('a\\nb\\nc'): want [a b c], got %q", got)
	}
}

func TestSplitLines_WhenLeadingNewline_ShouldOmitEmptyFirstLine(t *testing.T) {
	got := splitLines("\nfirst")
	if len(got) != 1 || got[0] != "first" {
		t.Errorf("splitLines('\\nfirst'): want [first], got %q", got)
	}
}

func TestStartup_WhenOptsNil_ShouldPrintToStdoutWithDelays(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	go func() {
		Startup("1.0.0", nil)
		w.Close()
		close(done)
	}()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	<-done

	out := buf.String()
	if !strings.Contains(out, "agent framework") || !strings.Contains(out, "v1.0.0") {
		t.Errorf("Startup(nil) output should contain banner and version: %q", out)
	}
}
