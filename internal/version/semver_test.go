package version

import "testing"

func TestSatisfies(t *testing.T) {
	tests := []struct {
		version     string
		constraints string
		want        bool
	}{
		{version: "1.2.3", constraints: ">=1.0.0,<2.0.0", want: true},
		{version: "2.0.0", constraints: ">=1.0.0,<2.0.0", want: false},
		{version: "1.2.3", constraints: "=1.2.3", want: true},
		{version: "1.2.4", constraints: "=1.2.3", want: false},
	}

	for _, test := range tests {
		got, err := Satisfies(test.version, test.constraints)
		if err != nil {
			t.Fatalf("Satisfies(%q, %q) returned error: %v", test.version, test.constraints, err)
		}
		if got != test.want {
			t.Fatalf("Satisfies(%q, %q) = %v, want %v", test.version, test.constraints, got, test.want)
		}
	}
}

func TestParseRejectsInvalidVersions(t *testing.T) {
	if _, err := Parse("1.2"); err == nil {
		t.Fatalf("expected Parse to reject invalid version")
	}
}
