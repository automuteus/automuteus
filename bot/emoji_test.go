package bot

import "testing"

func TestIsEmpty(t *testing.T) {
	e := emptyStatusEmojis()

	if !e.isEmpty() {
		t.Fatalf("fresh initialized emojis should be empty")
	}

	e = GlobalAlivenessEmojis

	if e.isEmpty() {
		t.Fatalf("valid emojis shouldn't report as empty")
	}
}
