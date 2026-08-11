package doctor

import "testing"

func TestParseJavaMajor(t *testing.T) {
	cases := map[string]int{
		`java version "17.0.9" 2023-10-17 LTS`: 17,
		`openjdk version "1.8.0_392"`:          8,
		`java version "11.0.2" 2019-01-15`:     11,
		`garbage`:                              0,
	}
	for in, want := range cases {
		if got := parseJavaMajor(in); got != want {
			t.Fatalf("%q: got %d want %d", in, got, want)
		}
	}
}
