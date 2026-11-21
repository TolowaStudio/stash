package brain

import "testing"

func testEpisodeOnlyConfig() Config {
	cfg := DefaultConfig()
	cfg.EpisodeOnlyNamespacePrefixes = ParseEpisodeOnlyNamespacePrefixes("")
	return cfg
}

func TestNamespaceIsEpisodeOnly_DefaultPrefixes(t *testing.T) {
	cfg := testEpisodeOnlyConfig()

	episodeOnly := []string{
		"/system/healthcheck",
		"/system/healthcheck/mc-canary",
		"/projects/fleet",
		"/projects/fleet/nodes/m2",
	}
	for _, ns := range episodeOnly {
		if !cfg.NamespaceIsEpisodeOnly(ns) {
			t.Errorf("expected episode-only namespace %q", ns)
		}
	}

	consolidate := []string{
		"/projects/x",
		"/projects/stash",
		"/system/other",
		"/",
	}
	for _, ns := range consolidate {
		if cfg.NamespaceIsEpisodeOnly(ns) {
			t.Errorf("expected consolidation namespace %q", ns)
		}
	}
}

func TestNamespaceIsEpisodeOnly_CustomPrefixList(t *testing.T) {
	cfg := Config{
		EpisodeOnlyNamespacePrefixes: []string{"/telemetry"},
	}
	if !cfg.NamespaceIsEpisodeOnly("/telemetry/run") {
		t.Fatal("expected /telemetry descendant to be episode-only")
	}
	if cfg.NamespaceIsEpisodeOnly("/projects/fleet") {
		t.Fatal("/projects/fleet should consolidate when not in prefix list")
	}
}

func TestParseEpisodeOnlyNamespacePrefixes_EmptyUsesDefault(t *testing.T) {
	got := ParseEpisodeOnlyNamespacePrefixes("")
	want := []string{"/system/healthcheck", "/projects/fleet"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestRunEpisodesToFactsConsolidation(t *testing.T) {
	cfg := testEpisodeOnlyConfig()

	if !cfg.RunEpisodesToFactsConsolidation("/projects/x") {
		t.Fatal("episodes in /projects/x must be consolidated into facts")
	}
	if cfg.RunEpisodesToFactsConsolidation("/system/healthcheck") {
		t.Fatal("episodes in /system/healthcheck must never be consolidated into facts")
	}
	if cfg.RunEpisodesToFactsConsolidation("/projects/fleet") {
		t.Fatal("episodes in /projects/fleet must never be consolidated into facts")
	}
}