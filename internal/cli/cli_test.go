package cli

import (
	"bytes"
	"testing"
)

func TestRun_WhenArgsContainNailPolishEmoji_ShouldPrintEmojiAndReturnZero(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"ironclaw", NailPolish}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	got := out.String()
	if got != NailPolish+"\n" {
		t.Errorf("expected stdout %q, got %q", NailPolish+"\n", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("expected no stderr, got %q", errOut.String())
	}
}

func TestRun_WhenArgsDoNotContainNailPolishEmoji_ShouldReturnOne(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"ironclaw", "serve"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if out.Len() != 0 {
		t.Errorf("expected no stdout, got %q", out.String())
	}
}

func TestRun_WhenArgsEmpty_ShouldReturnOne(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}
