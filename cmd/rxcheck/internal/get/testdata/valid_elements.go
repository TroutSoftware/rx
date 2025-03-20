package P

import "github.com/TroutSoftware/rx"

var _ = rx.Get(`<div></div>`)
var _ = rx.Get(`<div><span>Content</span></div>`)
var _ = rx.Get(`<div><br /></div>`)
var _ = rx.Get(`<span class="text-red">I can be '&lt;_&gt;' here</span>`)
var _ = rx.Get(`<div>&lt;span&gt;Content&lt;/span&gt;</div>`)
