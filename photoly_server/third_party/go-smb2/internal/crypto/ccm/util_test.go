package ccm

import (
	"bytes"
	"math"
	"strconv"
	"testing"
)

func TestSliceForAppendRejectsOverflow(t *testing.T) {
	for _, n := range []int{-1, math.MaxInt} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			defer func() {
				if got := recover(); got != "ccm: message too large" {
					t.Fatalf("overflow guard panic = %v", got)
				}
			}()
			sliceForAppend([]byte{1}, n)
		})
	}
}

func TestSliceForAppendPreservesPrefix(t *testing.T) {
	for _, capacity := range []int{3, 8} {
		prefix := make([]byte, 3, capacity)
		copy(prefix, []byte{1, 2, 3})
		head, tail := sliceForAppend(prefix, 2)
		copy(tail, []byte{4, 5})
		if !bytes.Equal(head, []byte{1, 2, 3, 4, 5}) {
			t.Fatalf("append = %v", head)
		}
		if capacity == 8 && &head[0] != &prefix[0] {
			t.Fatal("did not reuse available capacity")
		}
	}
}
