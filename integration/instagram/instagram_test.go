package instagram

import "testing"

func TestNormalizeInputURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://www.instagram.com/reel/ABC/", "https://www.instagram.com/reel/ABC/"},
		{"/reel/ABC/", "https://www.instagram.com/reel/ABC/"},
		{"reel/ABC/", "https://www.instagram.com/reel/ABC/"},
	}
	for _, tt := range tests {
		if got := normalizeInputURL(tt.in); got != tt.want {
			t.Errorf("normalizeInputURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGetShortcode(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://www.instagram.com/reel/C_BM2yAN4Rm/", "C_BM2yAN4Rm"},
		{"https://www.instagram.com/p/ABC123/?igsh=xyz", "ABC123"},
		{"https://www.instagram.com/reels/XYZ/", "XYZ"},
		{"https://www.instagram.com/tv/TVCODE/", "TVCODE"},
		{"/share/p/ABC/", "ABC"}, // share/p/<code> still exposes the shortcode
		{"https://www.instagram.com/share/xyz/", ""},
	}
	for _, tt := range tests {
		got, err := getShortcode(tt.in)
		if tt.want == "" {
			if err == nil {
				t.Errorf("getShortcode(%q) expected error, got %q", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("getShortcode(%q) unexpected err: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("getShortcode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
