package memory

import (
	"context"
	"fmt"
	"strings"

	agdomain "github.com/liliang-cn/agent-go/v2/pkg/domain"
	agmemory "github.com/liliang-cn/agent-go/v2/pkg/memory"
	agstore "github.com/liliang-cn/agent-go/v2/pkg/store"
)

// recallOverfetchFactor controls how many raw hits Recall asks the store for
// relative to the caller's limit. We over-fetch because the app-level scope
// filter (defense in depth against cross-repo leakage) may drop hits before we
// reach the requested count.
const recallOverfetchFactor = 3

// agentGoMemory is a Memory backed by agent-go's CortexDB memory service in
// lexical mode (no embedder, no LLM, no network). Search uses FTS5/BM25 with an
// n-gram/LIKE fallback, so recall is purely text-based and offline.
type agentGoMemory struct {
	svc *agmemory.Service
}

// NewAgentGo opens (or creates) a CortexDB memory store at dbPath and returns a
// Memory backed by agent-go's lexical memory service.
func NewAgentGo(dbPath string) (Memory, error) {
	st, err := agstore.NewCortexMemoryStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cortex memory store at %q: %w", dbPath, err)
	}
	svc := agmemory.NewService(st, nil /*llm*/, nil /*embedder*/, agmemory.DefaultConfig())
	return &agentGoMemory{svc: svc}, nil
}

func (m *agentGoMemory) Record(ctx context.Context, rec RunRecord) error {
	mem := &agdomain.Memory{
		Type:       agdomain.MemoryTypeObservation,
		ScopeType:  agdomain.MemoryScopeUser,
		ScopeID:    rec.Scope.Repo,
		Content:    renderRunRecord(rec),
		Tags:       []string{rec.Scope.Repo},
		Importance: 0.5,
		Metadata: map[string]any{
			"agent":   rec.Agent,
			"mode":    rec.Mode,
			"success": rec.Success,
			"kind":    "run",
		},
	}
	if err := m.svc.Add(ctx, mem); err != nil {
		return fmt.Errorf("record run for scope %q: %w", rec.Scope.Repo, err)
	}
	return nil
}

func (m *agentGoMemory) Note(ctx context.Context, scope Scope, fact string, tags []string) error {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return nil
	}
	allTags := make([]string, 0, len(tags)+1)
	allTags = append(allTags, scope.Repo)
	allTags = append(allTags, tags...)
	mem := &agdomain.Memory{
		Type:       agdomain.MemoryTypeFact,
		ScopeType:  agdomain.MemoryScopeUser,
		ScopeID:    scope.Repo,
		Content:    fact,
		Tags:       allTags,
		Importance: 0.7,
		Metadata: map[string]any{
			"kind": "note",
		},
	}
	if err := m.svc.Add(ctx, mem); err != nil {
		return fmt.Errorf("note fact for scope %q: %w", scope.Repo, err)
	}
	return nil
}

func (m *agentGoMemory) Recall(ctx context.Context, scope Scope, query string, limit int) (Recollection, error) {
	if limit <= 0 {
		limit = 5
	}
	hits, err := m.svc.Search(ctx, query, limit*recallOverfetchFactor)
	if err != nil {
		return Recollection{}, fmt.Errorf("recall for scope %q: %w", scope.Repo, err)
	}

	var rec Recollection
	for _, h := range hits {
		if h == nil || h.Memory == nil {
			continue
		}
		// Scope filter: memory must belong to this repo. Critical for isolation.
		if h.ScopeID != scope.Repo {
			continue
		}
		if len(rec.Episodes)+len(rec.Knowledge) >= limit {
			break
		}
		switch h.Type {
		case agdomain.MemoryTypeFact:
			rec.Knowledge = append(rec.Knowledge, Fact{
				Text: h.Content,
				Tags: h.Tags,
			})
		default:
			rec.Episodes = append(rec.Episodes, Episode{
				Summary:    h.Content,
				Agent:      metaString(h.Metadata, "agent"),
				Mode:       metaString(h.Metadata, "mode"),
				Success:    metaBool(h.Metadata, "success"),
				OccurredAt: h.CreatedAt,
			})
		}
	}

	rec.ContextText = renderRecollection(rec)
	return rec, nil
}

// renderRunRecord turns a RunRecord into a readable multi-line string suitable
// for lexical indexing and later display.
func renderRunRecord(rec RunRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run by %s in %s mode (success=%t)\n", rec.Agent, rec.Mode, rec.Success)
	if p := strings.TrimSpace(rec.Prompt); p != "" {
		fmt.Fprintf(&b, "Prompt: %s\n", p)
	}
	if r := strings.TrimSpace(rec.ResultSummary); r != "" {
		fmt.Fprintf(&b, "Result: %s\n", r)
	}
	if len(rec.ChangedPaths) > 0 {
		fmt.Fprintf(&b, "Changed: %s\n", strings.Join(rec.ChangedPaths, ", "))
	}
	if v := strings.TrimSpace(rec.Verdict); v != "" {
		fmt.Fprintf(&b, "Verdict: %s\n", v)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderRecollection builds the prompt-ready ContextText. Returns "" when there
// is nothing to inject.
func renderRecollection(r Recollection) string {
	if len(r.Episodes) == 0 && len(r.Knowledge) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Memory context (from past runs in this repo; advisory, may be stale):")
	for _, f := range r.Knowledge {
		fmt.Fprintf(&b, "\n- Known: %s", oneLine(f.Text))
	}
	for _, e := range r.Episodes {
		fmt.Fprintf(&b, "\n- %s", oneLine(e.Summary))
	}
	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func metaBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
