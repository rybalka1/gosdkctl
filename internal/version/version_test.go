package version

import "testing"

func TestIsGoVersionDir(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"go1.24.2": true,
		"go1.26":   true,
		"go":       false,
		"go1":      false,
		"go1.x":    false,
		"python3":  false,
	}

	for input, want := range cases {
		if got := IsGoVersionDir(input); got != want {
			t.Fatalf("IsGoVersionDir(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()

	if got := Compare("go1.25.1", "go1.26.1"); got >= 0 {
		t.Fatalf("expected go1.25.1 < go1.26.1, got %d", got)
	}

	if got := Compare("go1.26.1", "go1.26.1"); got != 0 {
		t.Fatalf("expected equal versions, got %d", got)
	}

	if got := Compare("go1.26.1", "go1.25.9"); got <= 0 {
		t.Fatalf("expected go1.26.1 > go1.25.9, got %d", got)
	}
}
