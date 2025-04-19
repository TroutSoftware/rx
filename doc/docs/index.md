# RX Framework for Web Applications in Go

Reactive engines, probably made well-known in the React.js framework, have rocked the UI world: the clear separation of the state change phase from the full-clean tree rendering phase helped untie some pretty gnarly code bases. One of my personal favorite benefit is on the refactoring side: no more subtle, time-sensitive dependency between components means you can independently modify the visual tree and the time ordering of widgets (including concurrent rendering when relevant).

When we started our journey building SecurityHub (our friendly cyber-security product), we knew we wanted all those benefits for a faster, simpler development experience. But we were also very cognizant of the limitation of React, and even Javascript (quadratic tree diffing algorithm, inability to control object allocations, …) when dealing with a large number of objects – which tends to be a common situation when dealing with any real-world cyber environment!

So we did what every rational person would do: we implemented our own reactive rendering engine. And, because of the tooling we loved around Go, we decided to stick with it in the front-end too. Using WASM.

And, probably to everyone’s surprised (at least mine!), we managed to get a minimal, yet solid prototype in two weeks; further small revisions and tooling improvements later, we have a fast, functional, and enjoyable engine to do front-end development clocking at ~2.5k lines of Go.

## Getting started

The framework currently requires a specific runtime to run within a page:
[Trout Go Runtime](https://github.com/TroutSoftware/go). We hope to upstream
[issue 56084](https://github.com/golang/go/issues/56084) soon and remove that step.

