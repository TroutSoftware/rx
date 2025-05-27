# Understanding State and Widgets

A key consideration for effective use of a reactive library is to know where state should be stored, and how to modify is.
The base idea is quite simple:
 - state is updated via an action (a function from state to state)
 - state is rendered via a widget tree.

Let’s unpack those two steps.

## State in the `Context`

A `rx.Context` is simply a map from types to values of the type.
If you want to display a user name, create a type for it, and store the initial value:

```go
type UserName string

// in your init function
rx.LoadContext(UserName("John Doe"))
```

The state can be accessed (during rendering or for actions) by the `rx.ValueOf` generic functions:

```go
// in a render loop
if rx.ValueOf[UserName]() != "" {
    rx.Get("<div>")
}
```

If no value is in present in the context, the zero value for its type will be returned instead,
so you do not have to worry about billion-dollar mistakes anymore.

While this is pretty much all there is about context and state, we have found this simplicity to be quite empowering.
Some of the types we often use in context:
 
  - dedicated errors types for specific parts of the UI. During rendering, the widget checks if an error of the right type is present in the context, and display it to the user in a friendly manner (e.g. using a custom pop-up).

  - `FormData` objects to represent input forms, and check inputs for logic before being stored in a dedicated object for display


## `Widgets` in a Tree

UI is rendered by a tree of widgets (implementing the interface of the same name):

```go
type Widget interface{ Build(Context) *Node }
```

For the simplest widgets (which only read state from the context), a shortcut type is also available:

```go
type WidgetFunc func(Context) *Node
```

Thus, one of the (possibly) simplest hello world widget would be:
```go
func helloweb(ctx Context) *Node { return Get("<p>Hello Web</p>")}
```

While true, this is not super interesting, so a dynamic widget would look instead like:

```go
func helloweb(ctx Context) *Node { 
    return Get("<p>").AddText(fmt.Sprintf("Hello %s", ValueOf[UserName]()))
}
```

The `Get` function is really the workhorse of rendering: `Node` objects are a simple transposition in Go of the DOM nodes (so they have a class, attributes, …), and one could construct a full node tree by hand.
But we think passing data as text is much easier, so instead we rely on Get to parse its arguments and construct a tree from it:

```go
Get(`<div class="bg-red">Hello</div>`)
```

is just another way of saying:

```go
GetNode("div").AddClasses("bg-red").AddText("Hello")
```

we do find the first one much easier to understand!

_Perf Note 1_: we use the `GetNode` method to initiate a node instead of creating a new node object.
 This is because nodes are managed with a bump allocator that gets cleared after each rendering cycle.
 Our internal benchmarks show we can usually have zero GC overhead in that phase.

_Perf Note 2_: parsing is only performed once for `Get` calls if the string does not change: 
 we maintain a cache of tree creation functions to minimize the runtime costs.
 We are investigating compiling ahead-of-time too, given most of the Get calls are effectively static.

- [ ] Include discussion about keep and drag / drop

## `Actions` Update the State

 - [ ] Show how to react to intents
 - [ ] Show how to update with back-end sync