package seed

import "testing"

func TestContainsDenied(t *testing.T) {
	tests := []struct {
		chinese string
		english string
		want    bool
	}{
		{sleepTraditional, sleepEnglish, false},
		{"他媽的！", "Damn it!", true},
		{"你好。", "What the fuck.", true},
		// Substring matching must not fire on innocent words.
		{"我住在薩塞克斯。", "I live in Sussex.", false},
	}

	for _, tt := range tests {
		if got := containsDenied(tt.chinese, tt.english); got != tt.want {
			t.Errorf("containsDenied(%q, %q) = %v, want %v", tt.chinese, tt.english, got, tt.want)
		}
	}
}
