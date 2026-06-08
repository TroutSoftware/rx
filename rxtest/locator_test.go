package rxtest

import (
	"testing"

	"github.com/TroutSoftware/rx"
)

func TestGetByRole(t *testing.T) {
	n := rx.Get("<article>").AddChildren(
		rx.Get("<h1>hello</h1>"),
	)
	r := Locate(Element{rxNode: n}, HasRole("heading", RoleOption{}))
	if !Expect(r, HasText("hello")) {
		t.Errorf("not found")
	}
}

func TestByTestID(t *testing.T) {
	n := rx.Get("<div>").AddChildren(
		rx.Get(`<button data-testid="submit">`),
		rx.Get(`<span data-testid="label">`),
	)

	r := Locate(Element{rxNode: n}, ByTestID("submit"))
	if r == notFound {
		t.Errorf("expected to find node with testid 'submit'")
	}

	r = Locate(Element{rxNode: n}, ByTestID("nonexistent"))
	if r != notFound {
		t.Errorf("expected not to find node with testid 'nonexistent'")
	}
}
