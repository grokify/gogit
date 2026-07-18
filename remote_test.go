package gogit

import "testing"

func TestNormalizeRemoteURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/x/y.git":  "github.com/x/y",
		"git@github.com:x/y.git":      "github.com/x/y",
		"ssh://git@github.com/x/y":    "github.com/x/y",
		"http://gitlab.com/a/b/c.git": "gitlab.com/a/b/c",
		"":                            "",
	}
	for in, want := range cases {
		if got := NormalizeRemoteURL(in); got != want {
			t.Errorf("NormalizeRemoteURL(%q): got %q, want %q", in, got, want)
		}
	}
}
