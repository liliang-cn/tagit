package feishu

import "testing"

func TestParseTextContentStripsMentions(t *testing.T) {
	got := parseTextContent(`{"text":"@_user_1  add input validation"}`)
	if got != "add input validation" {
		t.Fatalf("parseTextContent = %q", got)
	}
}

func TestParseTextContentPlain(t *testing.T) {
	if got := parseTextContent(`{"text":"hello world"}`); got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestParseTextContentNonText(t *testing.T) {
	if got := parseTextContent(`{"image_key":"img_x"}`); got != "" {
		t.Fatalf("non-text content should yield empty, got %q", got)
	}
}
