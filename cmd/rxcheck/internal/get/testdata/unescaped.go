package P

import "github.com/TroutSoftware/rx"

var _ = rx.Get(`<div>Content with < unescaped</div>`) // want "no escape characters in text use &lt, &gt, …"
var _ = rx.Get(`<div class="mt-2">>>>`)               // want "no escape characters in text use &lt, &gt, …"
