package bunker

import "testing"

func TestExtractClipID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://bnkr.in/clips/f0354b78-dad3-46ba-ae99-a5b26218acae", "f0354b78-dad3-46ba-ae99-a5b26218acae"},
		{"https://www.bnkr.in/clips/f0354b78-dad3-46ba-ae99-a5b26218acae/", "f0354b78-dad3-46ba-ae99-a5b26218acae"},
		{"https://bnkr.in/clips/f0354b78-dad3-46ba-ae99-a5b26218acae?foo=bar", "f0354b78-dad3-46ba-ae99-a5b26218acae"},
		{"https://bnkr.in/live/123", ""},
		{"https://bnkr.in/", ""},
	}
	for _, tt := range tests {
		got, err := ExtractClipID(tt.in)
		if tt.want == "" {
			if err == nil {
				t.Errorf("ExtractClipID(%q) expected error, got %q", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ExtractClipID(%q) unexpected err: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ExtractClipID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
