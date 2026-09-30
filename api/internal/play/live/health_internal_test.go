package live

import "testing"

func TestHealthIsWhatAnyoneCanSee(t *testing.T) {
	t.Parallel()
	for hp, want := range map[int]string{7: "unhurt", 6: "hurt", 4: "hurt", 3: "bloodied", 1: "bloodied", 0: "down", -2: "down"} {
		if got := health(hp, 7); got != want {
			t.Errorf("%d of 7 = %s, want %s", hp, got, want)
		}
	}
}
