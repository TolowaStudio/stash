package brain

import "testing"

func TestNormalizeNamespaceSlug(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"/projects/motion-control", "projectsmotioncontrol"},
		{"/projects/motion/control", "projectsmotioncontrol"},
		{"/clients/west-porch", "clientswestporch"},
		{"/clients/westporch", "clientswestporch"},
		{"/", ""},
	}
	for _, tt := range tests {
		if got := NormalizeNamespaceSlug(tt.slug); got != tt.want {
			t.Errorf("NormalizeNamespaceSlug(%q) = %q, want %q", tt.slug, got, tt.want)
		}
	}
}
