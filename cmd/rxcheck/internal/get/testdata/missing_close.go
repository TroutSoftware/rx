package P

import "github.com/TroutSoftware/rx"

var _ = rx.Get(`<div`)          // want "invalid element"
var _ = rx.Get(`<div class=""`) // want "invalid element"
var _ = rx.Get(`<div class=">`) // want "invalid element"
