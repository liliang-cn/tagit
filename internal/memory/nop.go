package memory

import "context"

type nopMemory struct{}

// Nop returns a Memory that stores nothing and recalls nothing. Used when memory
// is disabled or the backing engine fails to initialize.
func Nop() Memory { return nopMemory{} }

func (nopMemory) Recall(context.Context, Scope, string, int) (Recollection, error) {
	return Recollection{}, nil
}
func (nopMemory) Record(context.Context, RunRecord) error             { return nil }
func (nopMemory) Note(context.Context, Scope, string, []string) error { return nil }
