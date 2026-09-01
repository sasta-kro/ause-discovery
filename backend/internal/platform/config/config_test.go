package config

import "testing"

func TestNormalizePublicBasePath(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "root", input: "/", expected: "/"},
		{name: "default path", input: "/ause-discovery/", expected: "/ause-discovery/"},
		{name: "duplicate separators", input: "//ause-discovery//admin//", expected: "/ause-discovery/admin/"},
		{name: "missing separators", input: "ause-discovery", expected: "/ause-discovery/"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := NormalizePublicBasePath(testCase.input)
			if err != nil {
				t.Fatalf("NormalizePublicBasePath returned an error: %v", err)
			}
			if actual != testCase.expected {
				t.Fatalf("NormalizePublicBasePath = %q, expected %q", actual, testCase.expected)
			}
		})
	}
}

func TestNormalizePublicBasePathRejectsQueryAndFragment(t *testing.T) {
	for _, input := range []string{"/ause-discovery/?preview=1", "/ause-discovery/#section"} {
		if _, err := NormalizePublicBasePath(input); err == nil {
			t.Fatalf("NormalizePublicBasePath(%q) returned no error", input)
		}
	}
}
