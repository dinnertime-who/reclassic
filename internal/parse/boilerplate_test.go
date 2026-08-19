package parse

import "testing"

func TestStripBoilerplate(t *testing.T) {
	t.Parallel()

	raw := "license *** START OF THE PROJECT GUTENBERG EBOOK FOO *** hello *** END OF THE PROJECT GUTENBERG EBOOK FOO *** more license"
	body, start, end := StripBoilerplate(raw)
	if !start || !end {
		t.Fatalf("markers not found: start=%v end=%v", start, end)
	}
	if body != "hello" {
		t.Fatalf("body = %q, want hello", body)
	}

	old := "xxx *** START OF THIS PROJECT GUTENBERG EBOOK BAR *** body *** END OF THIS PROJECT GUTENBERG EBOOK BAR ***"
	body, start, end = StripBoilerplate(old)
	if !start || !end || body != "body" {
		t.Fatalf("old marker form: body=%q start=%v end=%v", body, start, end)
	}

	none := "<html><p>no markers here</p></html>"
	body, start, end = StripBoilerplate(none)
	if start || end || body != none {
		t.Fatalf("missing markers should pass through: %q %v %v", body, start, end)
	}
}
