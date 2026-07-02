package utils

import (
	"math"
	"math/rand"
	"testing"
)

func TestToBase62(t *testing.T) {
	for i := 0; i < 1000; i++ {
		d := rand.Int63n(math.MaxInt64)
		str := ToBase62(d)
		d1 := ToBase10(str)
		if d != d1 {
			t.Error("ToBase62 error")
		}
	}
}

func TestToBase10RejectsInvalidCharacters(t *testing.T) {
	if got := ToBase10("!"); got != 0 {
		t.Fatalf("ToBase10 invalid key = %d, want 0", got)
	}
}
