package P

import "github.com/TroutSoftware/rx"

var _ = rx.Get(`<div><p></p>`)             // want "unbalanced: div"
var _ = rx.Get(`<div><p></div>`)           // want "unmatched close token: div closing p"
var _ = rx.Get(`<div></div></div>`)        // want "extraneous close token: div"
var _ = rx.Get(`<div><span>Content</div>`) // want "unmatched close token: div closing span"
var _ = rx.Get(`<div><span></div>`)        // want "unmatched close token: div closing span"
var _ = rx.Get(`<div></span></div>`)       // want "unmatched close token: span closing div"
