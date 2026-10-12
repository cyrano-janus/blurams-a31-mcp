package blurams

import "testing"

func TestPasswordDigestKnownValue(t *testing.T) {
	if got := passwordDigest("secret"); got != "36cbca07d1617c974edd8c1bc978696c" {
		t.Fatalf("digest = %s", got)
	}
}

func TestRequestSignatureKnownValue(t *testing.T) {
	got := requestSignature("key", "123", "/path", "secret", "{}", "nonce")
	want := "0bdc1fee272f6cca99bbc74a3c2c34cbedd48841086396fbf7366a040cf382f6"
	if got != want {
		t.Fatalf("signature = %s", got)
	}
}

func TestRequestSignatureUsesEveryInput(t *testing.T) {
	base := []string{"key", "123", "/path", "secret", "{}", "nonce"}
	want := requestSignature(base[0], base[1], base[2], base[3], base[4], base[5])
	for i := range base {
		changed := append([]string(nil), base...)
		changed[i] += "-changed"
		got := requestSignature(changed[0], changed[1], changed[2], changed[3], changed[4], changed[5])
		if got == want {
			t.Fatalf("argument %d did not affect signature", i)
		}
	}
}

func TestCapabilities(t *testing.T) {
	tests := []struct {
		comment   string
		pan, tilt bool
	}{
		{`{'FEATURE': 48}`, true, true},
		{`{"FEATURE":16}`, true, false},
		{`{"FEATURE":32}`, false, true},
		{`{"FEATURE":1}`, false, false},
		{`{"FEATURE":33}`, false, true},
		{"missing", false, false},
	}
	for _, tc := range tests {
		pan, tilt := capabilities(tc.comment)
		if pan != tc.pan || tilt != tc.tilt {
			t.Fatalf("capabilities(%q) = %v,%v", tc.comment, pan, tilt)
		}
	}
}

func TestNormalizeMove(t *testing.T) {
	for direction, value := range map[string]int{"home": 0, "left": 1, "right": 2, "up": 3, "down": 4} {
		gotDirection, gotValue, err := normalizeMove(direction, 1)
		if err != nil || gotDirection != direction || gotValue != value {
			t.Fatalf("normalizeMove(%q) = %q,%d,%v", direction, gotDirection, gotValue, err)
		}
	}
	name, value, err := normalizeMove("RIGHT", 5)
	if err != nil || name != "right" || value != 2 {
		t.Fatalf("uppercase normalization failed: %q,%d,%v", name, value, err)
	}
}

func TestNormalizeMoveRejectsInvalidValues(t *testing.T) {
	for _, steps := range []int{-1, 0, 6} {
		if _, _, err := normalizeMove("left", steps); err == nil || err.Error() != "steps must be between 1 and 5" {
			t.Fatalf("steps %d: %v", steps, err)
		}
	}
	if _, _, err := normalizeMove("diagonal", 1); err == nil || err.Error() != "direction must be home, left, right, up, or down" {
		t.Fatalf("unexpected direction error: %v", err)
	}
}
