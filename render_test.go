package rx

import (
	"testing"
)

func TestRender(t *testing.T) {
	t.Run("create 2 different nodes", func(t *testing.T) {
		n1 := GetNode("span")
		if n1 == nil {
			t.Fatalf("n1 node is nil")
		}
		n2 := GetNode("span")
		if n2 == nil {
			t.Fatalf("n2 node is nil")
		}
		if n1 == n2 {
			t.Fatalf("n1 and n2 pointers are equal")
		}
	})
}
