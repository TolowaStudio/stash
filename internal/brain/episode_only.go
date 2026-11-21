package brain

import "strings"

const defaultEpisodeOnlyNamespacesCSV = "/system/healthcheck,/projects/fleet"

// ParseEpisodeOnlyNamespacePrefixes splits a comma-separated namespace prefix list.
func ParseEpisodeOnlyNamespacePrefixes(csv string) []string {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		csv = defaultEpisodeOnlyNamespacesCSV
	}
	parts := strings.Split(csv, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// NamespaceIsEpisodeOnly reports whether consolidation must not promote episodes to facts.
// Prefixes match the namespace slug itself and any descendant path (prefix + "/...").
func (c Config) NamespaceIsEpisodeOnly(namespaceSlug string) bool {
	for _, prefix := range c.EpisodeOnlyNamespacePrefixes {
		if namespaceSlug == prefix || strings.HasPrefix(namespaceSlug, prefix+"/") {
			return true
		}
	}
	return false
}

// RunEpisodesToFactsConsolidation is the stage-1 gate used by ConsolidateByID.
func (c Config) RunEpisodesToFactsConsolidation(namespaceSlug string) bool {
	return !c.NamespaceIsEpisodeOnly(namespaceSlug)
}