package conformance

import "testing"

func TestParseReportsStructuralErrorLocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "header", input: "nagi-fixture-v1\tother\n", want: "fixture.txt:1: unsupported or mismatched header"},
		{name: "duplicate", input: "nagi-fixture-v1\tsuite\ncase\tvalue=a\tvalue=b\n", want: "fixture.txt:2: duplicate field"},
		{name: "unknown", input: "nagi-fixture-v1\tsuite\ncase\tother=a\n", want: "fixture.txt:2: unknown field"},
		{name: "missing", input: "nagi-fixture-v1\tsuite\ncase\n", want: "fixture.txt:2: missing field"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := parse("fixture.txt", []byte(test.input), "suite", []string{"value"}); err == nil || err.Error() != test.want {
				t.Fatalf("parse() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDecodeRejectsInvalidUnicodeScalar(t *testing.T) {
	t.Parallel()

	if _, err := Decode(`\u{D800}`); err == nil || err.Error() != "invalid Unicode scalar value" {
		t.Fatalf("Decode() error = %v, want invalid Unicode scalar value", err)
	}
}
