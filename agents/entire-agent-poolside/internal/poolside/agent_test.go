package poolside

import "testing"

func TestSafeFilename(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"abc-123", "abc-123"}, {"", "unknown"}, {"a/b", "a_b"}} {
		if got := safeFilename(tc.in); got != tc.want {
			t.Fatalf("safeFilename(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
func TestFormatResumeCommand(t *testing.T) {
	a := New()
	if got := a.FormatResumeCommand("abc-123"); got != "pool -r abc-123" {
		t.Fatalf("got %q", got)
	}
	if got := a.FormatResumeCommand("a;rm"); got != "pool -r 'a;rm'" {
		t.Fatalf("got %q", got)
	}
}
