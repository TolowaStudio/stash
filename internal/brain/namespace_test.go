package brain

import "testing"

// The merge contract: target-after == source-before + target-before for every
// memory table. MemoryCounts.Total is the sum the merge check compares.
func TestMemoryCountsTotalSumsAllTables(t *testing.T) {
	c := MemoryCounts{Episodes: 1, Facts: 2, Relationships: 3, Patterns: 4, CausalLinks: 5,
		Goals: 6, Hypotheses: 7, Failures: 8, Contradictions: 9}
	if got := c.Total(); got != 45 {
		t.Fatalf("Total() = %d, want 45", got)
	}
}

// Every table the merge moves must be listed, or its rows would be stranded on the
// deleted source namespace (ON DELETE CASCADE would then erase them).
func TestMemoryTablesCoverAllNamespaceScopedMemories(t *testing.T) {
	want := map[string]bool{"episodes": true, "facts": true, "relationships": true, "patterns": true,
		"causal_links": true, "goals": true, "hypotheses": true, "failures": true, "contradictions": true}
	if len(memoryTables) != len(want) {
		t.Fatalf("memoryTables has %d entries, want %d", len(memoryTables), len(want))
	}
	for _, mt := range memoryTables {
		if !want[mt.name] {
			t.Errorf("unexpected table %q in memoryTables", mt.name)
		}
	}
}
