package feishu

import "testing"

func TestTextContentJSONEscapes(t *testing.T) {
	got := textContentJSON(`he said "hi"` + "\n" + "next")
	want := `{"text":"he said \"hi\"\nnext"}`
	if got != want {
		t.Fatalf("textContentJSON = %s, want %s", got, want)
	}
}
