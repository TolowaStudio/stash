package brain

import "strings"

// NormalizeNamespaceSlug lowercases and strips hyphens, underscores, slashes, and ".com"
// so split-brain twins like /projects/motion-control and /projects/motion/control collide.
func NormalizeNamespaceSlug(slug string) string {
	s := strings.ToLower(slug)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, ".com", "")
	return s
}