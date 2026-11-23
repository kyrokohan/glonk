// Package sitepages makes the pages of glonk.dev that are the repository's own text: the manual
// and a page for each guide from the files of docs, and the changelog from CHANGELOG.md. It is a
// test, like internal/brand: under make check it fails when a page is not what those files say, and
//
//	go test ./internal/sitepages -update
//
// (make site) writes the pages again.
package sitepages

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"
)

// A line of a Markdown file, with its number in that file for the errors.
type line struct {
	n    int
	text string
}

// A heading of a document, for its contents list.
type heading struct {
	level    int
	id, text string
}

// A renderer turns the Markdown that CHANGELOG.md and the guides use into HTML: headings,
// paragraphs, fenced code, flat lists, tables, and in a line code, bold, links and pictures. It
// knows that much and no more. Anything else is an error that names the line, so that a new kind
// of Markdown in those files is noticed here and not as a broken page.
type renderer struct {
	// link gives the address on the site of what a link or a picture in the file names.
	link func(target string) (string, error)
	// picture gives the HTML of a picture. linked says that it stands inside a link.
	picture func(alt, src string, linked bool) (string, error)

	refs     map[string]string // [0.12.0]: https://... at the foot of a file
	ids      map[string]int    // how often each heading id has been made
	headings []heading
	inLink   bool
}

var (
	headingLine = regexp.MustCompile(`^(#{1,3}) (\S.*)$`)
	refLine     = regexp.MustCompile(`^\[([^\]]+)\]: (\S+)$`)
	ruleRow     = regexp.MustCompile(`^\|( *:?-+:? *\|)+$`)
	// What Markdown would read as a block of a kind this renderer does not know: an indented line,
	// a quote, a numbered list, a list with * or +, a deeper heading, a rule, an underlined
	// heading, a fence of tildes.
	unknownBlock = regexp.MustCompile(`^(\s|>|\d+[.)] |[*+] |#|-{3,}$|={3,}$|~~~)`)
)

// render turns lines into HTML. The headings it meets are added to r.headings.
func (r *renderer) render(lines []line) (string, error) {
	if r.refs == nil {
		r.refs, r.ids = map[string]string{}, map[string]int{}
	}
	// The targets of reference links stand anywhere in the file, usually at its foot.
	fenced := false
	for _, l := range lines {
		if strings.HasPrefix(l.text, "```") {
			fenced = !fenced
		}
		if m := refLine.FindStringSubmatch(l.text); m != nil && !fenced {
			r.refs[m[1]] = m[2]
		}
	}

	var out strings.Builder
	for i := 0; i < len(lines); {
		l := lines[i]
		switch {
		case l.text == "", refLine.MatchString(l.text):
			i++

		case strings.HasPrefix(l.text, "```"):
			end := i + 1
			for end < len(lines) && lines[end].text != "```" {
				end++
			}
			if end == len(lines) {
				return "", fmt.Errorf("line %d: the code block is not closed", l.n)
			}
			var code []string
			for _, c := range lines[i+1 : end] {
				code = append(code, c.text)
			}
			out.WriteString(codeBlock(code))
			i = end + 1

		case headingLine.MatchString(l.text):
			m := headingLine.FindStringSubmatch(l.text)
			inner, text, err := r.inline(l.n, m[2])
			if err != nil {
				return "", err
			}
			id := r.id(text)
			if id == "" {
				return "", fmt.Errorf("line %d: the heading %q has nothing to make an id from", l.n, m[2])
			}
			r.headings = append(r.headings, heading{level: len(m[1]), id: id, text: text})
			fmt.Fprintf(&out, "<h%d id=\"%s\">%s</h%d>\n", len(m[1]), id, inner, len(m[1]))
			i++

		case strings.HasPrefix(l.text, "- "):
			out.WriteString("<ul>\n")
			for ; i < len(lines) && strings.HasPrefix(lines[i].text, "- "); i++ {
				inner, _, err := r.inline(lines[i].n, lines[i].text[2:])
				if err != nil {
					return "", err
				}
				out.WriteString("<li>" + inner + "</li>\n")
			}
			out.WriteString("</ul>\n")

		case strings.HasPrefix(l.text, "|"):
			end := i
			for end < len(lines) && strings.HasPrefix(lines[end].text, "|") {
				end++
			}
			table, err := r.table(lines[i:end])
			if err != nil {
				return "", err
			}
			out.WriteString(table)
			i = end

		case unknownBlock.MatchString(l.text):
			return "", fmt.Errorf("line %d: a kind of Markdown that internal/sitepages does not know: %q", l.n, clip(l.text))

		default:
			// A paragraph runs to a blank line or to the start of another block.
			end := i + 1
			for end < len(lines) && lines[end].text != "" && !startsBlock(lines[end].text) {
				end++
			}
			out.WriteString("<p>")
			for j, p := range lines[i:end] {
				inner, _, err := r.inline(p.n, p.text)
				if err != nil {
					return "", err
				}
				if j > 0 {
					out.WriteString("\n")
				}
				out.WriteString(inner)
			}
			out.WriteString("</p>\n")
			i = end
		}
	}
	return out.String(), nil
}

// codeBlock is the HTML of lines of code. Code stands on rows half as tall as the page's, so a
// block of an odd number of lines ends half a row short. The stylesheet makes that up for a
// block marked odd.
func codeBlock(code []string) string {
	odd := ""
	if len(code)%2 == 1 {
		odd = ` class="odd"`
	}
	var out strings.Builder
	out.WriteString(`<pre tabindex="0"` + odd + `><code>`)
	for _, c := range code {
		out.WriteString(html.EscapeString(c) + "\n")
	}
	out.WriteString("</code></pre>\n")
	return out.String()
}

// startsBlock reports whether a line ends the paragraph above it.
func startsBlock(text string) bool {
	// A heading starts with #, which unknownBlock has too.
	return strings.HasPrefix(text, "```") || strings.HasPrefix(text, "- ") || strings.HasPrefix(text, "|") ||
		refLine.MatchString(text) || unknownBlock.MatchString(text)
}

// table turns the rows of a pipe table into HTML. A head row of empty cells, which Markdown needs
// and a table of pictures has no use for, is left out.
func (r *renderer) table(rows []line) (string, error) {
	if len(rows) < 3 || !ruleRow.MatchString(rows[1].text) {
		return "", fmt.Errorf("line %d: a table needs a head row, a row of dashes and a row under them", rows[0].n)
	}
	cells := func(l line) []string {
		parts := strings.Split(strings.Trim(strings.TrimSpace(l.text), "|"), "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	head := cells(rows[0])
	var out strings.Builder
	out.WriteString("<table>\n")
	row := func(l line, tag string) error {
		cs := cells(l)
		// A | inside a cell, in code say, would be read as the end of the cell.
		if len(cs) != len(head) {
			return fmt.Errorf("line %d: the row has %d cells and the head of its table has %d", l.n, len(cs), len(head))
		}
		out.WriteString("<tr>")
		for _, c := range cs {
			inner, _, err := r.inline(l.n, c)
			if err != nil {
				return err
			}
			out.WriteString("<" + tag + ">" + inner + "</" + tag + ">")
		}
		out.WriteString("</tr>\n")
		return nil
	}
	if strings.Join(head, "") != "" {
		out.WriteString("<thead>\n")
		if err := row(rows[0], "th"); err != nil {
			return "", err
		}
		out.WriteString("</thead>\n")
	}
	out.WriteString("<tbody>\n")
	for _, l := range rows[2:] {
		if err := row(l, "td"); err != nil {
			return "", err
		}
	}
	out.WriteString("</tbody>\n</table>\n")
	return out.String(), nil
}

// inline turns the Markdown inside a line into HTML, and also gives its plain text, which a
// heading's id and the contents list are made from.
func (r *renderer) inline(n int, s string) (markup, text string, err error) {
	var m, t strings.Builder
	for len(s) > 0 {
		switch {
		case s[0] == '`':
			end := strings.IndexByte(s[1:], '`')
			if end < 0 {
				return "", "", fmt.Errorf("line %d: a ` that is not closed", n)
			}
			code := s[1 : 1+end]
			m.WriteString("<code>" + html.EscapeString(code) + "</code>")
			t.WriteString(code)
			s = s[end+2:]

		case strings.HasPrefix(s, "**"):
			end := find(s[2:], "**")
			if end < 0 {
				return "", "", fmt.Errorf("line %d: a ** that is not closed", n)
			}
			im, it, err := r.inline(n, s[2:2+end])
			if err != nil {
				return "", "", err
			}
			m.WriteString("<strong>" + im + "</strong>")
			t.WriteString(it)
			s = s[end+4:]

		case strings.HasPrefix(s, "!["):
			alt, target, rest, ok := bracketed(s[1:])
			if !ok {
				return "", "", fmt.Errorf("line %d: a picture is written ![what it shows](its file)", n)
			}
			p, err := r.picture(alt, target, r.inLink)
			if err != nil {
				return "", "", fmt.Errorf("line %d: %w", n, err)
			}
			m.WriteString(p)
			s = rest

		case s[0] == '[':
			label, target, rest, ok := bracketed(s)
			if !ok {
				// A reference link: [0.12.0], with its target on a line of its own.
				end := find(s[1:], "]")
				if end < 0 || r.refs[s[1:1+end]] == "" {
					return "", "", fmt.Errorf("line %d: a [ that starts neither a link nor a reference this file gives a target for", n)
				}
				label, target, rest = s[1:1+end], r.refs[s[1:1+end]], s[end+2:]
			}
			href, err := r.link(target)
			if err != nil {
				return "", "", fmt.Errorf("line %d: %w", n, err)
			}
			if r.inLink {
				return "", "", fmt.Errorf("line %d: a link inside a link", n)
			}
			r.inLink = true
			im, it, err := r.inline(n, label)
			r.inLink = false
			if err != nil {
				return "", "", err
			}
			m.WriteString(`<a href="` + html.EscapeString(href) + `">` + im + "</a>")
			t.WriteString(it)
			s = rest

		case strings.HasPrefix(s, "<br>"):
			m.WriteString("<br>")
			s = s[4:]

		case s[0] == '\\', s[0] == '*', len(s) > 1 && s[0] == '<' && (s[1] == '/' || s[1] == '!' || unicode.IsLetter(rune(s[1]))):
			return "", "", fmt.Errorf("line %d: a kind of Markdown that internal/sitepages does not know (HTML other than <br>, a backslash, or emphasis with one *): %q", n, clip(s))

		default:
			// Plain text, up to the next character that may start something.
			end := strings.IndexAny(s[1:], "`*[<\\!")
			if end < 0 {
				end = len(s) - 1
			}
			m.WriteString(html.EscapeString(s[:end+1]))
			t.WriteString(s[:end+1])
			s = s[end+1:]
		}
	}
	return m.String(), t.String(), nil
}

// bracketed reads [label](target) from the start of s and gives what follows it. The label may
// hold brackets of its own, as a link round a picture does.
func bracketed(s string) (label, target, rest string, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return "", "", "", false
	}
	depth, end := 0, -1
	for i := 0; i < len(s) && end < 0; i++ {
		switch s[i] {
		case '`': // code is not looked into
			skip := strings.IndexByte(s[i+1:], '`')
			if skip < 0 {
				return "", "", "", false
			}
			i += skip + 1
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				end = i
			}
		}
	}
	if end < 0 || !strings.HasPrefix(s[end+1:], "(") {
		return "", "", "", false
	}
	closing := strings.IndexByte(s[end+2:], ')')
	if closing < 0 {
		return "", "", "", false
	}
	return s[1:end], s[end+2 : end+2+closing], s[end+3+closing:], true
}

// find is strings.Index that does not look into code: the ** of a bold run that has `a ** b` in
// it is the one after the code.
func find(s, sep string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			skip := strings.IndexByte(s[i+1:], '`')
			if skip < 0 {
				return -1
			}
			i += skip + 1
			continue
		}
		if strings.HasPrefix(s[i:], sep) {
			return i
		}
	}
	return -1
}

// id makes the id of a heading the way GitHub does for the same heading in the file, so that a
// link to a section works in both places: lower case, with letters, digits, hyphens and
// underscores kept, spaces made hyphens and the rest dropped, and a number after one made before.
func (r *renderer) id(text string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(c), unicode.IsDigit(c), c == '-', c == '_':
			b.WriteRune(c)
		case c == ' ':
			b.WriteByte('-')
		}
	}
	id := b.String()
	seen := r.ids[id]
	r.ids[id]++
	if seen > 0 {
		id = fmt.Sprintf("%s-%d", id, seen)
	}
	return id
}

// clip shortens a line for an error.
func clip(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "..."
	}
	return s
}
