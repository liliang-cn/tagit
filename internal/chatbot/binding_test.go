package chatbot

import "testing"

func TestBindingsForHitMiss(t *testing.T) {
	bs := Bindings{
		{ChatID: "c1", Repo: "/r1", Agent: "codex", Mode: "rage"},
		{ChatID: "c2", Repo: "/r2"},
	}
	b, ok := bs.For("c1")
	if !ok || b.Repo != "/r1" || b.Agent != "codex" || b.Mode != "rage" {
		t.Fatalf("For(c1) = %+v ok=%v", b, ok)
	}
	if _, ok := bs.For("nope"); ok {
		t.Fatal("For(nope) must miss")
	}
}
