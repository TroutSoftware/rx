// Original code from golang.org/x/net/html
// Original license file in LICENSE_xnet

package rx

import (
	"bytes"
	"io"
	"strconv"
	"strings"
)

// A TokenType is the type of a Token.
type TokenType uint32

const (
	// errorToken means that an error occurred during tokenization.
	errorToken TokenType = iota
	// textToken means a text node.
	textToken
	// A startTagToken looks like <a>.
	startTagToken
	// An endTagToken looks like </a>.
	endTagToken
	// A selfClosingTagToken tag looks like <br/>.
	selfClosingTagToken
	// A commentToken looks like <!--x-->.
	commentToken
	// A doctypeToken looks like <!DOCTYPE x>
	doctypeToken
)

// String returns a string representation of the TokenType.
func (t TokenType) String() string {
	switch t {
	case errorToken:
		return "Error"
	case textToken:
		return "Text"
	case startTagToken:
		return "StartTag"
	case endTagToken:
		return "EndTag"
	case selfClosingTagToken:
		return "SelfClosingTag"
	case commentToken:
		return "Comment"
	case doctypeToken:
		return "Doctype"
	}
	return "Invalid(" + strconv.Itoa(int(t)) + ")"
}

// span is a range of bytes in a Tokenizer's buffer. The start is inclusive,
// the end is exclusive.
type span struct {
	Start, End int
}

func (s span) String() string { return "[" + strconv.Itoa(s.Start) + ":" + strconv.Itoa(s.End) + "]" }
func (s span) Len() int       { return s.End - s.Start }

// A tokenizer returns a stream of HTML Tokens.
type tokenizer struct {
	// tt is the TokenType of the current token.
	tt TokenType
	// err is the first error encountered during tokenization. It is possible
	// for tt != Error && err != nil to hold: this means that Next returned a
	// valid token but the subsequent Next call will return an error token.
	// For example, if the HTML text input was just "plain", then the first
	// Next call would set z.err to io.EOF but return a TextToken, and all
	// subsequent Next calls would return an ErrorToken.
	// err is never reset. Once it becomes non-nil, it stays non-nil.
	err error
	// buf[raw.start:raw.end] holds the raw bytes of the current token.
	// buf[raw.end:] is buffered input that will yield future tokens.
	raw span
	buf []byte
	// buf[data.start:data.end] holds the raw bytes of the current token's data:
	// a text token's text, a tag token's tag name, etc.
	data span
	// pendingAttr is the attribute key and value currently being tokenized.
	// When complete, pendingAttr is pushed onto attr. nAttrReturned is
	// incremented on each call to TagAttr.
	pendingAttr   [2]span
	attr          [][2]span
	nAttrReturned int
	// rawTag is the "script" in "</script>" that closes the next token. If
	// non-empty, the subsequent call to Next will return a raw or RCDATA text
	// token: one that treats "<p>" as text instead of an element.
	// rawTag's contents are lower-cased.
	rawTag string
	// textIsRaw is whether the current text token's data is not escaped.
	textIsRaw bool
	// convertNUL is whether NUL bytes in the current token's data should
	// be converted into \ufffd replacement characters.
	convertNUL bool
	// allowCDATA is whether CDATA sections are allowed in the current context.
	allowCDATA bool
}

// AllowCDATA sets whether or not the tokenizer recognizes <![CDATA[foo]]> as
// the text "foo". The default value is false, which means to recognize it as
// a bogus comment "<!-- [CDATA[foo]] -->" instead.
//
// Strictly speaking, an HTML5 compliant tokenizer should allow CDATA if and
// only if tokenizing foreign content, such as MathML and SVG. However,
// tracking foreign-contentness is difficult to do purely in the tokenizer,
// as opposed to the parser, due to HTML integration points: an <svg> element
// can contain a <foreignObject> that is foreign-to-SVG but not foreign-to-
// HTML. For strict compliance with the HTML5 tokenization algorithm, it is the
// responsibility of the user of a tokenizer to call AllowCDATA as appropriate.
// In practice, if using the tokenizer without caring whether MathML or SVG
// CDATA is text or comments, such as tokenizing HTML to find all the anchor
// text, it is acceptable to ignore this responsibility.
func (z *tokenizer) AllowCDATA(allowCDATA bool) {
	z.allowCDATA = allowCDATA
}

// NextIsNotRawText instructs the tokenizer that the next token should not be
// considered as 'raw text'. Some elements, such as script and title elements,
// normally require the next token after the opening tag to be 'raw text' that
// has no child elements. For example, tokenizing "<title>a<b>c</b>d</title>"
// yields a start tag token for "<title>", a text token for "a<b>c</b>d", and
// an end tag token for "</title>". There are no distinct start tag or end tag
// tokens for the "<b>" and "</b>".
//
// This tokenizer implementation will generally look for raw text at the right
// times. Strictly speaking, an HTML5 compliant tokenizer should not look for
// raw text if in foreign content: <title> generally needs raw text, but a
// <title> inside an <svg> does not. Another example is that a <textarea>
// generally needs raw text, but a <textarea> is not allowed as an immediate
// child of a <select>; in normal parsing, a <textarea> implies </select>, but
// one cannot close the implicit element when parsing a <select>'s InnerHTML.
// Similarly to AllowCDATA, tracking the correct moment to override raw-text-
// ness is difficult to do purely in the tokenizer, as opposed to the mdt.
// For strict compliance with the HTML5 tokenization algorithm, it is the
// responsibility of the user of a tokenizer to call NextIsNotRawText as
// appropriate. In practice, like AllowCDATA, it is acceptable to ignore this
// responsibility for basic usage.
//
// Note that this 'raw text' concept is different from the one offered by the
// Tokenizer.Raw method.
func (z *tokenizer) NextIsNotRawText() {
	z.rawTag = ""
}

// Err returns the error associated with the most recent ErrorToken token.
// This is typically io.EOF, meaning the end of tokenization.
func (z *tokenizer) Err() error {
	if z.tt != errorToken {
		return nil
	}
	return z.err
}

// readByte returns the next byte from the input stream, doing a buffered read
// from z.r into z.buf if necessary. z.buf[z.raw.start:z.raw.end] remains a contiguous byte
// slice that holds all the bytes read so far for the current token.
// It sets z.err if the underlying reader returns an error.
// Pre-condition: z.err == nil.
func (z *tokenizer) readByte() byte {
	if z.raw.End == len(z.buf) {
		z.err = io.EOF
		return '\x00'
	}
	x := z.buf[z.raw.End]
	z.raw.End++
	return x
}

// skipWhiteSpace skips past any white space.
func (z *tokenizer) skipWhiteSpace() {
	if z.err != nil {
		return
	}
	for {
		c := z.readByte()
		if z.err != nil {
			return
		}
		switch c {
		case ' ', '\n', '\r', '\t', '\f':
			// No-op.
		default:
			z.raw.End--
			return
		}
	}
}

// readRawOrRCDATA reads until the next "</foo>", where "foo" is z.rawTag and
// is typically something like "script" or "textarea".
func (z *tokenizer) readRawOrRCDATA() {
	if z.rawTag == "script" {
		z.readScript()
		z.textIsRaw = true
		z.rawTag = ""
		return
	}
loop:
	for {
		c := z.readByte()
		if z.err != nil {
			break loop
		}
		if c != '<' {
			continue loop
		}
		c = z.readByte()
		if z.err != nil {
			break loop
		}
		if c != '/' {
			z.raw.End--
			continue loop
		}
		if z.readRawEndTag() || z.err != nil {
			break loop
		}
	}
	z.data.End = z.raw.End
	// A textarea's or title's RCDATA can contain escaped entities.
	z.textIsRaw = z.rawTag != "textarea" && z.rawTag != "title"
	z.rawTag = ""
}

// readRawEndTag attempts to read a tag like "</foo>", where "foo" is z.rawTag.
// If it succeeds, it backs up the input position to reconsume the tag and
// returns true. Otherwise it returns false. The opening "</" has already been
// consumed.
func (z *tokenizer) readRawEndTag() bool {
	for i := 0; i < len(z.rawTag); i++ {
		c := z.readByte()
		if z.err != nil {
			return false
		}
		if c != z.rawTag[i] && c != z.rawTag[i]-('a'-'A') {
			z.raw.End--
			return false
		}
	}
	c := z.readByte()
	if z.err != nil {
		return false
	}
	switch c {
	case ' ', '\n', '\r', '\t', '\f', '/', '>':
		// The 3 is 2 for the leading "</" plus 1 for the trailing character c.
		z.raw.End -= 3 + len(z.rawTag)
		return true
	}
	z.raw.End--
	return false
}

// readScript reads until the next </script> tag, following the byzantine
// rules for escaping/hiding the closing tag.
func (z *tokenizer) readScript() {
	defer func() {
		z.data.End = z.raw.End
	}()
	var c byte

scriptData:
	c = z.readByte()
	if z.err != nil {
		return
	}
	if c == '<' {
		goto scriptDataLessThanSign
	}
	goto scriptData

scriptDataLessThanSign:
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case '/':
		goto scriptDataEndTagOpen
	case '!':
		goto scriptDataEscapeStart
	}
	z.raw.End--
	goto scriptData

scriptDataEndTagOpen:
	if z.readRawEndTag() || z.err != nil {
		return
	}
	goto scriptData

scriptDataEscapeStart:
	c = z.readByte()
	if z.err != nil {
		return
	}
	if c == '-' {
		goto scriptDataEscapeStartDash
	}
	z.raw.End--
	goto scriptData

scriptDataEscapeStartDash:
	c = z.readByte()
	if z.err != nil {
		return
	}
	if c == '-' {
		goto scriptDataEscapedDashDash
	}
	z.raw.End--
	goto scriptData

scriptDataEscaped:
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case '-':
		goto scriptDataEscapedDash
	case '<':
		goto scriptDataEscapedLessThanSign
	}
	goto scriptDataEscaped

scriptDataEscapedDash:
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case '-':
		goto scriptDataEscapedDashDash
	case '<':
		goto scriptDataEscapedLessThanSign
	}
	goto scriptDataEscaped

scriptDataEscapedDashDash:
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case '-':
		goto scriptDataEscapedDashDash
	case '<':
		goto scriptDataEscapedLessThanSign
	case '>':
		goto scriptData
	}
	goto scriptDataEscaped

scriptDataEscapedLessThanSign:
	c = z.readByte()
	if z.err != nil {
		return
	}
	if c == '/' {
		goto scriptDataEscapedEndTagOpen
	}
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
		goto scriptDataDoubleEscapeStart
	}
	z.raw.End--
	goto scriptData

scriptDataEscapedEndTagOpen:
	if z.readRawEndTag() || z.err != nil {
		return
	}
	goto scriptDataEscaped

scriptDataDoubleEscapeStart:
	z.raw.End--
	for i := 0; i < len("script"); i++ {
		c = z.readByte()
		if z.err != nil {
			return
		}
		if c != "script"[i] && c != "SCRIPT"[i] {
			z.raw.End--
			goto scriptDataEscaped
		}
	}
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case ' ', '\n', '\r', '\t', '\f', '/', '>':
		goto scriptDataDoubleEscaped
	}
	z.raw.End--
	goto scriptDataEscaped

scriptDataDoubleEscaped:
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case '-':
		goto scriptDataDoubleEscapedDash
	case '<':
		goto scriptDataDoubleEscapedLessThanSign
	}
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapedDash:
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case '-':
		goto scriptDataDoubleEscapedDashDash
	case '<':
		goto scriptDataDoubleEscapedLessThanSign
	}
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapedDashDash:
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case '-':
		goto scriptDataDoubleEscapedDashDash
	case '<':
		goto scriptDataDoubleEscapedLessThanSign
	case '>':
		goto scriptData
	}
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapedLessThanSign:
	c = z.readByte()
	if z.err != nil {
		return
	}
	if c == '/' {
		goto scriptDataDoubleEscapeEnd
	}
	z.raw.End--
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapeEnd:
	if z.readRawEndTag() {
		z.raw.End += len("</script>")
		goto scriptDataEscaped
	}
	if z.err != nil {
		return
	}
	goto scriptDataDoubleEscaped
}

// readComment reads the next comment token starting with "<!--". The opening
// "<!--" has already been consumed.
func (z *tokenizer) readComment() {
	// When modifying this function, consider manually increasing the
	// maxSuffixLen constant in func TestComments, from 6 to e.g. 9 or more.
	// That increase should only be temporary, not committed, as it
	// exponentially affects the test running time.

	z.data.Start = z.raw.End
	defer func() {
		if z.data.End < z.data.Start {
			// It's a comment with no data, like <!-->.
			z.data.End = z.data.Start
		}
	}()

	var dashCount int
	beginning := true
	for {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.calculateAbruptCommentDataEnd()
			return
		}
		switch c {
		case '-':
			dashCount++
			continue
		case '>':
			if dashCount >= 2 || beginning {
				z.data.End = z.raw.End - len("-->")
				return
			}
		case '!':
			if dashCount >= 2 {
				c = z.readByte()
				if z.err != nil {
					z.data.End = z.calculateAbruptCommentDataEnd()
					return
				} else if c == '>' {
					z.data.End = z.raw.End - len("--!>")
					return
				} else if c == '-' {
					dashCount = 1
					beginning = false
					continue
				}
			}
		}
		dashCount = 0
		beginning = false
	}
}

func (z *tokenizer) calculateAbruptCommentDataEnd() int {
	raw := z.buf[z.raw.Start:z.raw.End]
	const prefixLen = len("<!--")
	if len(raw) >= prefixLen {
		raw = raw[prefixLen:]
		if hasSuffix(raw, "--!") {
			return z.raw.End - 3
		} else if hasSuffix(raw, "--") {
			return z.raw.End - 2
		} else if hasSuffix(raw, "-") {
			return z.raw.End - 1
		}
	}
	return z.raw.End
}

func hasSuffix(b []byte, suffix string) bool {
	if len(b) < len(suffix) {
		return false
	}
	b = b[len(b)-len(suffix):]
	for i := range b {
		if b[i] != suffix[i] {
			return false
		}
	}
	return true
}

// readUntilCloseAngle reads until the next ">".
func (z *tokenizer) readUntilCloseAngle() {
	z.data.Start = z.raw.End
	for {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return
		}
		if c == '>' {
			z.data.End = z.raw.End - len(">")
			return
		}
	}
}

// readMarkupDeclaration reads the next token starting with "<!". It might be
// a "<!--comment-->", a "<!DOCTYPE foo>", a "<![CDATA[section]]>" or
// "<!a bogus comment". The opening "<!" has already been consumed.
func (z *tokenizer) readMarkupDeclaration() TokenType {
	z.data.Start = z.raw.End
	var c [2]byte
	for i := 0; i < 2; i++ {
		c[i] = z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return commentToken
		}
	}
	if c[0] == '-' && c[1] == '-' {
		z.readComment()
		return commentToken
	}
	z.raw.End -= 2
	if z.readDoctype() {
		return doctypeToken
	}
	if z.allowCDATA && z.readCDATA() {
		z.convertNUL = true
		return textToken
	}
	// It's a bogus comment.
	z.readUntilCloseAngle()
	return commentToken
}

// readDoctype attempts to read a doctype declaration and returns true if
// successful. The opening "<!" has already been consumed.
func (z *tokenizer) readDoctype() bool {
	const s = "DOCTYPE"
	for i := 0; i < len(s); i++ {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return false
		}
		if c != s[i] && c != s[i]+('a'-'A') {
			// Back up to read the fragment of "DOCTYPE" again.
			z.raw.End = z.data.Start
			return false
		}
	}
	if z.skipWhiteSpace(); z.err != nil {
		z.data.Start = z.raw.End
		z.data.End = z.raw.End
		return true
	}
	z.readUntilCloseAngle()
	return true
}

// readCDATA attempts to read a CDATA section and returns true if
// successful. The opening "<!" has already been consumed.
func (z *tokenizer) readCDATA() bool {
	const s = "[CDATA["
	for i := 0; i < len(s); i++ {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return false
		}
		if c != s[i] {
			// Back up to read the fragment of "[CDATA[" again.
			z.raw.End = z.data.Start
			return false
		}
	}
	z.data.Start = z.raw.End
	brackets := 0
	for {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return true
		}
		switch c {
		case ']':
			brackets++
		case '>':
			if brackets >= 2 {
				z.data.End = z.raw.End - len("]]>")
				return true
			}
			brackets = 0
		default:
			brackets = 0
		}
	}
}

// startTagIn returns whether the start tag in z.buf[z.data.start:z.data.end]
// case-insensitively matches any element of ss.
func (z *tokenizer) startTagIn(ss ...string) bool {
loop:
	for _, s := range ss {
		if z.data.End-z.data.Start != len(s) {
			continue loop
		}
		for i := 0; i < len(s); i++ {
			c := z.buf[z.data.Start+i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != s[i] {
				continue loop
			}
		}
		return true
	}
	return false
}

// readStartTag reads the next start tag token. The opening "<a" has already
// been consumed, where 'a' means anything in [A-Za-z].
func (z *tokenizer) readStartTag() TokenType {
	z.readTag(true)
	if z.err != nil {
		return errorToken
	}
	// Several tags flag the tokenizer's next token as raw.
	c, raw := z.buf[z.data.Start], false
	if 'A' <= c && c <= 'Z' {
		c += 'a' - 'A'
	}
	switch c {
	case 'i':
		raw = z.startTagIn("iframe")
	case 'n':
		raw = z.startTagIn("noembed", "noframes", "noscript")
	case 'p':
		raw = z.startTagIn("plaintext")
	case 's':
		raw = z.startTagIn("script", "style")
	case 't':
		raw = z.startTagIn("textarea", "title")
	case 'x':
		raw = z.startTagIn("xmp")
	}
	if raw {
		z.rawTag = strings.ToLower(string(z.buf[z.data.Start:z.data.End]))
	}
	// Look for a self-closing token like "<br/>".
	if z.err == nil && z.buf[z.raw.End-2] == '/' {
		return selfClosingTagToken
	}
	return startTagToken
}

// readTag reads the next tag token and its attributes. If saveAttr, those
// attributes are saved in z.attr, otherwise z.attr is set to an empty slice.
// The opening "<a" or "</a" has already been consumed, where 'a' means anything
// in [A-Za-z].
func (z *tokenizer) readTag(saveAttr bool) {
	z.attr = z.attr[:0]
	z.nAttrReturned = 0
	// Read the tag name and attribute key/value pairs.
	z.readTagName()
	if z.skipWhiteSpace(); z.err != nil {
		return
	}
	for {
		c := z.readByte()
		if z.err != nil || c == '>' {
			break
		}
		z.raw.End--
		z.readTagAttrKey()
		z.readTagAttrVal()
		// Save pendingAttr if saveAttr and that attribute has a non-empty key.
		if saveAttr && z.pendingAttr[0].Start != z.pendingAttr[0].End {
			z.attr = append(z.attr, z.pendingAttr)
		}
		if z.skipWhiteSpace(); z.err != nil {
			break
		}
	}
}

// readTagName sets z.data to the "div" in "<div k=v>". The reader (z.raw.end)
// is positioned such that the first byte of the tag name (the "d" in "<div")
// has already been consumed.
func (z *tokenizer) readTagName() {
	z.data.Start = z.raw.End - 1
	for {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return
		}
		switch c {
		case ' ', '\n', '\r', '\t', '\f':
			z.data.End = z.raw.End - 1
			return
		case '/', '>':
			z.raw.End--
			z.data.End = z.raw.End
			return
		}
	}
}

// readTagAttrKey sets z.pendingAttr[0] to the "k" in "<div k=v>".
// Precondition: z.err == nil.
func (z *tokenizer) readTagAttrKey() {
	z.pendingAttr[0].Start = z.raw.End
	for {
		c := z.readByte()
		if z.err != nil {
			z.pendingAttr[0].End = z.raw.End
			return
		}
		switch c {
		case ' ', '\n', '\r', '\t', '\f', '/':
			z.pendingAttr[0].End = z.raw.End - 1
			return
		case '=', '>':
			z.raw.End--
			z.pendingAttr[0].End = z.raw.End
			return
		}
	}
}

// readTagAttrVal sets z.pendingAttr[1] to the "v" in "<div k=v>".
func (z *tokenizer) readTagAttrVal() {
	z.pendingAttr[1].Start = z.raw.End
	z.pendingAttr[1].End = z.raw.End
	if z.skipWhiteSpace(); z.err != nil {
		return
	}
	c := z.readByte()
	if z.err != nil {
		return
	}
	if c != '=' {
		z.raw.End--
		return
	}
	if z.skipWhiteSpace(); z.err != nil {
		return
	}
	quote := z.readByte()
	if z.err != nil {
		return
	}
	switch quote {
	case '>':
		z.raw.End--
		return

	case '\'', '"':
		z.pendingAttr[1].Start = z.raw.End
		for {
			c := z.readByte()
			if z.err != nil {
				z.pendingAttr[1].End = z.raw.End
				return
			}
			if c == quote {
				z.pendingAttr[1].End = z.raw.End - 1
				return
			}
		}

	default:
		z.pendingAttr[1].Start = z.raw.End - 1
		for {
			c := z.readByte()
			if z.err != nil {
				z.pendingAttr[1].End = z.raw.End
				return
			}
			switch c {
			case ' ', '\n', '\r', '\t', '\f':
				z.pendingAttr[1].End = z.raw.End - 1
				return
			case '>':
				z.raw.End--
				z.pendingAttr[1].End = z.raw.End
				return
			}
		}
	}
}

// Next scans the next token and returns its type.
func (z *tokenizer) Next() TokenType {
	z.raw.Start = z.raw.End
	z.data.Start = z.raw.End
	z.data.End = z.raw.End
	if z.err != nil {
		z.tt = errorToken
		return z.tt
	}
	if z.rawTag != "" {
		if z.rawTag == "plaintext" {
			// Read everything up to EOF.
			for z.err == nil {
				z.readByte()
			}
			z.data.End = z.raw.End
			z.textIsRaw = true
		} else {
			z.readRawOrRCDATA()
		}
		if z.data.End > z.data.Start {
			z.tt = textToken
			z.convertNUL = true
			return z.tt
		}
	}
	z.textIsRaw = false
	z.convertNUL = false

loop:
	for {
		c := z.readByte()
		if z.err != nil {
			break loop
		}
		if c != '<' {
			continue loop
		}

		// Check if the '<' we have just read is part of a tag, comment
		// or doctype. If not, it's part of the accumulated text token.
		c = z.readByte()
		if z.err != nil {
			break loop
		}
		var tokenType TokenType
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			tokenType = startTagToken
		case c == '/':
			tokenType = endTagToken
		case c == '!' || c == '?':
			// We use CommentToken to mean any of "<!--actual comments-->",
			// "<!DOCTYPE declarations>" and "<?xml processing instructions?>".
			tokenType = commentToken
		default:
			// Reconsume the current character.
			z.raw.End--
			continue
		}

		// We have a non-text token, but we might have accumulated some text
		// before that. If so, we return the text first, and return the non-
		// text token on the subsequent call to Next.
		if x := z.raw.End - len("<a"); z.raw.Start < x {
			z.raw.End = x
			z.data.End = x
			z.tt = textToken
			return z.tt
		}
		switch tokenType {
		case startTagToken:
			z.tt = z.readStartTag()
			return z.tt
		case endTagToken:
			c = z.readByte()
			if z.err != nil {
				break loop
			}
			if c == '>' {
				// "</>" does not generate a token at all. Generate an empty comment
				// to allow passthrough clients to pick up the data using Raw.
				// Reset the tokenizer state and start again.
				z.tt = commentToken
				return z.tt
			}
			if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
				z.readTag(false)
				if z.err != nil {
					z.tt = errorToken
				} else {
					z.tt = endTagToken
				}
				return z.tt
			}
			z.raw.End--
			z.readUntilCloseAngle()
			z.tt = commentToken
			return z.tt
		case commentToken:
			if c == '!' {
				z.tt = z.readMarkupDeclaration()
				return z.tt
			}
			z.raw.End--
			z.readUntilCloseAngle()
			z.tt = commentToken
			return z.tt
		}
	}
	if z.raw.Start < z.raw.End {
		z.data.End = z.raw.End
		z.tt = textToken
		return z.tt
	}
	z.tt = errorToken
	return z.tt
}

func checkEscape(s []byte) {
	// https://www.w3.org/International/questions/qa-escapes#use
	// we tolerate single quote since this is only in double-quoted values
	// TODO(rdo) check for & too, when used in escaped values
	const escapedChars = "<>\""
	if bytes.ContainsAny(s, escapedChars) {
		panic("no escape characters in attribute or text use &lt, &gt, …")
	}
}

func checkLower(s []byte) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			panic("only use lower case attributes and tags")
		}
	}
}

func (z *tokenizer) Raw() span { return z.raw }

// Text returns the unescaped text of a text, comment or doctype token. The
// contents of the returned slice may change on the next call to Next.
func (z *tokenizer) Text() span {
	switch z.tt {
	case textToken, commentToken, doctypeToken:
		checkEscape(z.buf[z.data.Start:z.data.End])
		dt := z.data
		z.data.Start = z.raw.End
		z.data.End = z.raw.End
		return dt
	}
	return span{}
}

func (z *tokenizer) TagName() (name span, hasAttr bool) {
	if z.data.Start < z.data.End {
		switch z.tt {
		case startTagToken, endTagToken, selfClosingTagToken:
			checkLower(z.buf[z.data.Start:z.data.End])
			dt := z.data
			z.data.Start = z.raw.End
			z.data.End = z.raw.End
			return dt, z.nAttrReturned < len(z.attr)
		}
	}
	return span{}, false
}

func (z *tokenizer) TagAttr() (key, val span, moreAttr bool) {
	if z.nAttrReturned < len(z.attr) {
		switch z.tt {
		case startTagToken, selfClosingTagToken:
			x := z.attr[z.nAttrReturned]
			z.nAttrReturned++
			checkEscape(z.buf[x[1].Start:x[1].End])
			return x[0], x[1], z.nAttrReturned < len(z.attr)
		}
	}
	return span{}, span{}, false
}

func (z *tokenizer) TokenType() TokenType { return z.tt }

// newTokenizer returns a new HTML Tokenizer for the given Reader.
// The input is assumed to be UTF-8 encoded.
func newTokenizer(src string) *tokenizer {
	return &tokenizer{buf: []byte(src)}
}
