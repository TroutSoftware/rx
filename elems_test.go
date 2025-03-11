package rx

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestHTML(t *testing.T) {
	cases := []struct {
		tpl  string
		want *Node
	}{
		{`<div>`, GetNode("div")},
		{`<p role="label">`, GetNode("p").AddRole("label")},
		{`<div class="flex">I can be &lt here<button>Click me</button></div>`,
			GetNode("div").AddClasses("flex").
				SetText("I can be &lt here").
				AddChildren(GetNode("button").SetText("Click me"))},
		{`<div class="flex">I can be &lt;tag&gt; here<button>Click me</button></div>`,
			GetNode("div").AddClasses("flex").
				SetText("I can be <tag> here").
				AddChildren(GetNode("button").SetText("Click me"))},
		{`<div><div></div><div></div></div>`, GetNode("div").AddChildren(GetNode("div"), GetNode("div"))},
		{`<svg><path/></svg>`, GetNode("svg").AddChildren(GetNode("path"))},
	}

	pubfields := cmpopts.IgnoreUnexported(Node{})

	for _, c := range cases {
		got := Get(c.tpl)

		if !cmp.Equal(got, c.want, pubfields) {
			t.Errorf("in %s: %s", c.tpl, cmp.Diff(got, c.want, pubfields))
		}

		// run checks in serialize
		serialize(got, new(etree), new(Counter), make(XAS, 0))
	}
}

func TestUnescape(t *testing.T) {
	cases := []struct {
		input  string
		output string
	}{
		{"&lt;p&gt;This is a paragraph.&lt;/p&gt;", "<p>This is a paragraph.</p>"},
		{"&lt;a href=&quot;https://www.example.com&quot;&gt;Link&lt;/a&gt;", `<a href="https://www.example.com">Link</a>`},
		{"&lt;script&gt;alert(&quot;Hello!&quot;);&lt;/script&gt;", `<script>alert("Hello!");</script>`},
		{"&lt;style&gt;body { color: red; }&lt;/style&gt;", `<style>body { color: red; }</style>`},
		{"This &amp; that.", "This & that."},
		{"&quot;Quoted text&quot;", `"Quoted text"`},
		{"It&apos;s a test.", "It's a test."},
		{"Nested &lt;b&gt;bold&lt;/b&gt; and &lt;i&gt;italic&lt;/i&gt; text.", "Nested <b>bold</b> and <i>italic</i> text."},
		{"This &lt;em&gt;is not fully &lt;/em&gt; a test.", "This <em>is not fully </em> a test."},
	}

	for _, tc := range cases {
		got := unescape(tc.input)
		if got != tc.output {
			t.Errorf("unescape(%s) = %s; want %s", tc.input, got, tc.output)
		}
	}
}

func BenchmarkHTMLSimpleDiv(b *testing.B) {
	b.Run("quick-get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			FreePool()
			for i := 0; i < 3000; i++ {
				n := Get(`<div class="w-2 bg-zinc-200">`)
				// prevent compiler to skip the work
				if n == nil {
					b.Fatal("invalid node returned")
				}
			}
		}
	})
	b.Run("getnode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.ReportAllocs()
			FreePool()
			for i := 0; i < 3000; i++ {
				n := GetNode("div").AddClasses("w-2 bg-zinc-200")
				// prevent compiler to skip the work
				if n == nil {
					b.Fatal("invalid node returned")
				}
			}
		}
	})
}
