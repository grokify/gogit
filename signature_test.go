package gogit

import "testing"

func TestParseSignedHashes(t *testing.T) {
	raw := "commit aaa\ntree t\nauthor a\ncommitter c\ngpgsig -----BEGIN SSH SIGNATURE-----\n U1NI\n -----END SSH SIGNATURE-----\n\n    signed message\n" +
		"commit bbb\ntree t\nauthor a\ncommitter c\n\n    mentions it\n    gpgsig -----BEGIN\n    commit ccc\n"
	got := parseSignedHashes(raw)
	if len(got) != 2 || !got["aaa"] || got["bbb"] {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestSignatureHelpers(t *testing.T) {
	yes, no := true, false
	signed := Commit{Signed: &yes}
	unsigned := Commit{Signed: &no}
	unknown := Commit{}

	if !unsigned.IsUnsigned() || signed.IsUnsigned() || unknown.IsUnsigned() {
		t.Error("IsUnsigned must be true only for a known-unsigned commit")
	}
	if unknown.HasSignature() || !signed.HasSignature() {
		t.Error("HasSignature mismatch")
	}
	if HasSignatureInfo([]Commit{unknown}) || !HasSignatureInfo([]Commit{unknown, signed}) {
		t.Error("HasSignatureInfo mismatch")
	}
	if n := CountUnsigned([]Commit{signed, unsigned, unknown, unsigned}); n != 2 {
		t.Errorf("CountUnsigned = %d, want 2", n)
	}
}

func TestSignatureWarning(t *testing.T) {
	yes, no := true, false
	signed, unsigned := Commit{Signed: &yes}, Commit{Signed: &no}

	tests := []struct {
		name    string
		commits []Commit
		want    string
	}{
		{"none", nil, ""},
		{"all signed", []Commit{signed, signed}, ""},
		{"single unsigned is not warned", []Commit{unsigned}, ""},
		{"some unsigned", []Commit{signed, unsigned, unsigned}, "2 of 3 commits are not signed"},
		{"state not collected", []Commit{{}, {}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SignatureWarning(tt.commits); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if got := (PendingResult{Commits: tt.commits}).UnsignedWarning(); got != tt.want {
				t.Errorf("PendingResult.UnsignedWarning got %q, want %q", got, tt.want)
			}
		})
	}
}
