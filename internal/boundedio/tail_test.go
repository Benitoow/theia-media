package boundedio

import "testing"

func TestTailKeepsBoundedNewestBytes(t *testing.T) {
	buffer := NewTail(5)
	for _, part := range []string{"ab", "cdef", "gh"} {
		if n, err := buffer.Write([]byte(part)); err != nil || n != len(part) {
			t.Fatalf("Write(%q) = %d, %v", part, n, err)
		}
	}
	if got := buffer.String(); got != "defgh" {
		t.Fatalf("tail = %q, want defgh", got)
	}
}
