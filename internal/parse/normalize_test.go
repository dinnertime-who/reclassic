package parse

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "html entity then quotes and dashes",
			in:   "He said &ldquo;hello&rdquo; &mdash; then left.",
			want: `He said "hello" - then left.`,
		},
		{
			name: "curly quotes and en dash",
			in:   "It’s a ‘test’ – really",
			want: "It's a 'test' - really",
		},
		{
			name: "newlines tabs and spaces",
			in:   "  one\n\ttwo   three  ",
			want: "one two three",
		},
		{
			name: "nfc composed",
			in:   "cafe\u0301",
			want: "caf\u00e9",
		},
		{
			name: "does not fold case",
			in:   "Mr. Darcy",
			want: "Mr. Darcy",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Normalize(tc.in)
			if got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStableIDDeterministic(t *testing.T) {
	t.Parallel()
	text := Normalize(`He said “hello” — then left.`)
	a := StableID(text)
	b := StableID(text)
	if a != b {
		t.Fatalf("StableID not deterministic: %s vs %s", a, b)
	}
	if len(a) != 16 {
		t.Fatalf("StableID length = %d, want 16", len(a))
	}
	other := StableID(Normalize("different paragraph"))
	if a == other {
		t.Fatal("different text produced the same stable_id")
	}
}
