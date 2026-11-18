package sitepages

import (
	"os"
	"strings"
	"testing"
)

// md makes the lines of a Markdown file from its text.
func md(text string) []line {
	var out []line
	for i, t := range strings.Split(text, "\n") {
		out = append(out, line{n: i + 1, text: t})
	}
	return out
}

// plain is a renderer whose links and pictures go nowhere on disk.
func plain() *renderer {
	return &renderer{
		link: func(target string) (string, error) { return "to:" + target, nil },
		picture: func(alt, src string, linked bool) (string, error) {
			if linked {
				return "<img " + src + " linked>", nil
			}
			return "<img " + src + ">", nil
		},
	}
}

func TestRender(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"a heading gets the id GitHub gives it", "## Tile maps and a camera", "<h2 id=\"tile-maps-and-a-camera\">Tile maps and a camera</h2>\n"},
		{"punctuation is dropped from an id, hyphens and underscores kept", "### Moved to glonk.dev, re-do_it!", "<h3 id=\"moved-to-glonkdev-re-do_it\">Moved to glonk.dev, re-do_it!</h3>\n"},
		{"a heading made twice is numbered", "### Added\n### Added\n### Added", "<h3 id=\"added\">Added</h3>\n<h3 id=\"added-1\">Added</h3>\n<h3 id=\"added-2\">Added</h3>\n"},
		{"code and bold in a heading are in its id", "# The **pixel** `Canvas`", "<h1 id=\"the-pixel-canvas\">The <strong>pixel</strong> <code>Canvas</code></h1>\n"},
		{"a paragraph", "One & two < three.", "<p>One &amp; two &lt; three.</p>\n"},
		{"a paragraph of two lines", "One\ntwo.\n\nThree.", "<p>One\ntwo.</p>\n<p>Three.</p>\n"},
		{"code is not read as Markdown", "Use `a ** b` and `<br>` and `[x](y)`.", "<p>Use <code>a ** b</code> and <code>&lt;br&gt;</code> and <code>[x](y)</code>.</p>\n"},
		{"bold, with code in it", "**The camera is a `Rect`.** It moves.", "<p><strong>The camera is a <code>Rect</code>.</strong> It moves.</p>\n"},
		{"bold ends after code that holds two stars", "**a `x ** y` b** c", "<p><strong>a <code>x ** y</code> b</strong> c</p>\n"},
		{"a link", "See [the **list**](https://example.com/a?b=1&c=2).", "<p>See <a href=\"to:https://example.com/a?b=1&amp;c=2\">the <strong>list</strong></a>.</p>\n"},
		{"a link with a bracket in code", "[`a[`](#x)!", "<p><a href=\"to:#x\"><code>a[</code></a>!</p>\n"},
		{"a reference link", "## [0.12.0] - 2026-11-17\n\n[0.12.0]: https://example.com/compare", "<h2 id=\"0120---2026-11-17\"><a href=\"to:https://example.com/compare\">0.12.0</a> - 2026-11-17</h2>\n"},
		{"a picture", "![A block.](site/media/a.png)", "<p><img site/media/a.png></p>\n"},
		{"a picture after words", "See ![A block.](site/media/a.png)", "<p>See <img site/media/a.png></p>\n"},
		{"a paragraph ends where another block starts", "One\n## Two\nThree\n| a |\n|---|\n| b |\nFour\n```\nx\n```\nFive\n[r]: https://example.com", "<p>One</p>\n<h2 id=\"two\">Two</h2>\n<p>Three</p>\n<table>\n<thead>\n<tr><th>a</th></tr>\n</thead>\n<tbody>\n<tr><td>b</td></tr>\n</tbody>\n</table>\n<p>Four</p>\n<pre tabindex=\"0\"><code>x\n</code></pre>\n<p>Five</p>\n"},
		{"a picture in a link", "[![A block.](site/media/a.gif)](site/media/a.gif)", "<p><a href=\"to:site/media/a.gif\"><img site/media/a.gif linked></a></p>\n"},
		{"an exclamation mark is only that", "Stop! Now.", "<p>Stop! Now.</p>\n"},
		{"a code block", "```go\nif a < b && c {\n\treturn `x`\n}\n\n**not bold**\n```", "<pre tabindex=\"0\"><code>if a &lt; b &amp;&amp; c {\n\treturn `x`\n}\n\n**not bold**\n</code></pre>\n"},
		{"a reference inside a code block is code", "```\n[a]: b\n```\n[a]", ""},
		{"a reference after a code block", "```\nx\n```\n[a]\n\n[a]: https://example.com", "<pre tabindex=\"0\"><code>x\n</code></pre>\n<p><a href=\"to:https://example.com\">a</a></p>\n"},
		{"a list", "- one `1`\n- **two**", "<ul>\n<li>one <code>1</code></li>\n<li><strong>two</strong></li>\n</ul>\n"},
		{"a list right under a paragraph", "**engine**\n- one", "<p><strong>engine</strong></p>\n<ul>\n<li>one</li>\n</ul>\n"},
		{"a table", "| Keys | Action |\n|---|:--|\n| W / S | move<br>back |", "<table>\n<thead>\n<tr><th>Keys</th><th>Action</th></tr>\n</thead>\n<tbody>\n<tr><td>W / S</td><td>move<br>back</td></tr>\n</tbody>\n</table>\n"},
		{"a table with an empty head has none", "| | |\n|---|---|\n| a | b |", "<table>\n<tbody>\n<tr><td>a</td><td>b</td></tr>\n</tbody>\n</table>\n"},
	} {
		got, err := plain().render(md(c.in))
		if c.want == "" {
			if err == nil {
				t.Errorf("%s: wanted an error, got %q", c.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
		} else if got != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, c.want)
		}
	}
}

// What the renderer does not know it refuses, and says where. Quiet wrong HTML would be worse:
// the page is committed and served.
func TestRenderRefusesWhatItDoesNotKnow(t *testing.T) {
	for _, c := range []struct{ name, in, where string }{
		{"a nested list", "- one\n  - two", "line 2"},
		{"a numbered list", "Text.\n\n1. one", "line 3"},
		{"a list with a star", "* one", "line 1"},
		{"a quote", "> Early.", "line 1"},
		{"a quote right under a paragraph", "One.\n> two", "line 2"},
		{"an underlined heading", "Title\n===", "line 2"},
		{"a fence of tildes", "~~~\ncode\n~~~", "line 1"},
		{"a heading of the fourth level", "#### Deep", "line 1"},
		{"a rule", "One.\n\n---", "line 3"},
		{"an indented line", "    code", "line 1"},
		{"HTML", "A <b>bold</b> word.", "line 1"},
		{"an HTML block", "<details>", "line 1"},
		{"a comment", "<!-- hidden -->", "line 1"},
		{"emphasis with one star", "An *odd* word.", "line 1"},
		{"a backslash", `A C:\dir path.`, "line 1"},
		{"a closing tag", "A </b> word.", "line 1"},
		{"a footnote", "A note.[^1]", "line 1"},
		{"a reference with no target", "See [the list].", "line 1"},
		{"a code span that is not closed", "A `word.", "line 1"},
		{"a code span that is not closed, in bold", "**A `word.**", "line 1"},
		{"bold that is not closed", "A **word.", "line 1"},
		{"a code block that is not closed", "One.\n\n```go\nx := 1", "line 3"},
		{"a link that is not closed", "[a](b", "line 1"},
		{"a link in a link", "[a [b](c) d](e)", "line 1"},
		{"a picture with no file", "![alt]", "line 1"},
		{"a table with no row of dashes", "| a | b |\n| c | d |\n| e | f |", "line 1"},
		{"a table with no rows", "| a | b |\n|---|---|", "line 1"},
		{"a table row with a cell too many", "| a | b |\n|---|---|\n| c | d | e |", "line 3"},
		{"a table row with a cell too few", "| a | b |\n|---|---|\n| c | d |\n| e |", "line 4"},
		{"a heading with nothing to name it by", "## ...", "line 1"},
	} {
		got, err := plain().render(md(c.in))
		if err == nil {
			t.Errorf("%s: wanted an error, got %q", c.name, got)
		} else if !strings.Contains(err.Error(), c.where) {
			t.Errorf("%s: wanted the error to name %s, got %v", c.name, c.where, err)
		}
	}
}

// An error from a link or a picture names the line it is on.
func TestRenderSaysWhereALinkOrAPictureFails(t *testing.T) {
	r := plain()
	r.link = link
	r.picture = picture
	for _, in := range []string{"One.\n\n[a](no/such/file.md)", "One.\n\n![a](site/media/no-such.png)"} {
		if _, err := r.render(md(in)); err == nil || !strings.Contains(err.Error(), "line 3") {
			t.Errorf("%q: wanted an error that names line 3, got %v", in, err)
		}
	}
}

func TestSections(t *testing.T) {
	file := md("# Title\n\nAbove.\n\n## One\n\n1\n\n```\n## Not a heading\n```\n\n## Development\n\ndev\n\n## Two\n\n2")
	above, kept, err := sections(file, []string{"Development"})
	if err != nil {
		t.Fatal(err)
	}
	text := func(ls []line) string {
		var out []string
		for _, l := range ls {
			out = append(out, l.text)
		}
		return strings.Join(out, "\n")
	}
	if got, want := text(above), "# Title\n\nAbove.\n"; got != want {
		t.Errorf("above the first section:\ngot  %q\nwant %q", got, want)
	}
	if got, want := text(kept), "## One\n\n1\n\n```\n## Not a heading\n```\n\n## Two\n\n2"; got != want {
		t.Errorf("the sections kept:\ngot  %q\nwant %q", got, want)
	}
	if kept[0].n != 5 || kept[len(kept)-1].n != 19 {
		t.Errorf("wanted the lines to keep their numbers in the file, 5 to 19, got %d to %d", kept[0].n, kept[len(kept)-1].n)
	}

	// A section that was renamed would be in the manual under its new name, unnoticed.
	if _, _, err := sections(file, []string{"Development", "License"}); err == nil || !strings.Contains(err.Error(), "License") {
		t.Errorf("wanted an error that names the section that is not there, got %v", err)
	}
	// A heading inside a code block is code, and is not a section to leave out.
	if _, _, err := sections(file, []string{"Not a heading"}); err == nil {
		t.Error("wanted an error for a section that is only a line of a code block")
	}
}

func TestLink(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"#widgets", "#widgets"},
		{"https://pkg.go.dev/glonk.dev", "https://pkg.go.dev/glonk.dev"},
		{"LICENSE", repository + "/blob/main/LICENSE"},
		{"CONTRIBUTING.md#making-a-release", repository + "/blob/main/CONTRIBUTING.md#making-a-release"},
		{"examples/viewer", repository + "/blob/main/examples/viewer"},
		{"site/style.css", "/style.css"},
		{"no/such/file.md", ""},
		{"http://example.com", ""},
		{"mailto:someone", ""},
		{"/etc/hosts", ""},
		{"internal/../LICENSE", ""}, // a path that leaves the repository would be sent to GitHub as it is
	} {
		got, err := link(c.in)
		if (err != nil) != (c.want == "") || got != c.want {
			t.Errorf("link(%q): wanted %q, got %q and %v", c.in, c.want, got, err)
		}
	}
}

func TestPicture(t *testing.T) {
	if _, err := os.Stat(root("site/media")); os.IsNotExist(err) {
		t.Skip("site/media is not in this copy of glonk")
	}
	still := `<img src="/media/cube3d.png" width="840" height="560" alt="A &lt;block&gt;.">`
	for _, c := range []struct {
		src    string
		linked bool
		want   string
	}{
		{"site/media/cube3d.png", false, still},
		{"site/media/cube3d.png", true, still},
		// A recording is shown as its still, which opens it, unless a link round it already does.
		{"site/media/cube3d.gif", false, `<a href="/media/cube3d.gif">` + still + `</a>`},
		{"site/media/cube3d.gif", true, still},
		{"site/media/no-such.gif", false, ""},
		{"site/media/go.mod", false, ""}, // not a picture
		{"brand/mark.png", false, ""},    // not a file of the site
	} {
		got, err := picture("A <block>.", c.src, c.linked)
		if (err != nil) != (c.want == "") || got != c.want {
			t.Errorf("picture(%q, linked %v):\ngot  %q and %v\nwant %q", c.src, c.linked, got, err, c.want)
		}
	}
}

// The contents of a page list its sections, the ## headings, with their text made safe.
func TestTheContentsListTheSections(t *testing.T) {
	out, err := page{Contents: []heading{{2, "a--b", "A & B"}, {3, "deeper", "Deeper"}}}.write()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, `<li><a href="#a--b">A &amp; B</a></li>`) || strings.Contains(got, "deeper") {
		t.Errorf("wanted the contents to list the one section, escaped, got:\n%s", got)
	}
}

// The manual has every section of the README but the three about the repository, and each page
// lists its sections.
func TestThePagesHoldTheirSections(t *testing.T) {
	if _, err := os.Stat(root("site/media")); os.IsNotExist(err) {
		t.Skip("site/media is not in this copy of glonk")
	}
	readme, err := os.ReadFile(root("README.md"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := manual()
	if err != nil {
		t.Fatal(err)
	}
	out, err := m.write()
	if err != nil {
		t.Fatal(err)
	}
	listed := strings.Count(string(out), `<li><a href="#`)
	if want := strings.Count("\n"+string(readme), "\n## ") - len(notInTheManual); listed != want || listed == 0 {
		t.Errorf("wanted the manual's contents to list %d sections, got %d", want, listed)
	}
	for _, name := range notInTheManual {
		if strings.Contains(string(out), ">"+name+"</h2>") {
			t.Errorf("wanted the manual without the section %q", name)
		}
	}

	c, err := changelog()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Contents) == 0 || c.Contents[0].text != "Unreleased" || !strings.Contains(c.Front, `<h1 id="changelog">Changelog</h1>`) || !strings.Contains(c.Front, `<p class="lede">`) {
		t.Errorf("wanted the changelog to open with its title and its first paragraph and to list Unreleased first, got %q and %+v", c.Front, c.Contents[:min(1, len(c.Contents))])
	}
}
