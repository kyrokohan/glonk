package sitepages

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"image"
	_ "image/png"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
)

var update = flag.Bool("update", false, "write the pages again from the guides in docs and CHANGELOG.md")

const repository = "https://github.com/usrgames/glonk"

// A page of the site.
type page struct {
	Path        string // of the file, from the root of the repository
	Address     string // on glonk.dev
	Title       string
	Description string
	Front       string // above the contents: the h1 and what stands under it
	Contents    []heading
	Text        string

	// A guide, a page of the manual made from a file of docs, links the guide before it and the
	// one after it in the order of docs/README.md.
	Guide         bool
	Before, After *neighbor
}

// A guide beside another, as the shell links it.
type neighbor struct{ Address, Title string }

// The shell of a page: the head, the top bar and the foot of site/index.html, round the contents,
// where a page has them, and the text. The go-import and go-source tags come first, as internal/sitecheck wants of every
// page.
var shell = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="go-import" content="glonk.dev git https://github.com/usrgames/glonk">
<meta name="go-source" content="glonk.dev https://github.com/usrgames/glonk https://github.com/usrgames/glonk/tree/main{/dir} https://github.com/usrgames/glonk/blob/main{/dir}/{file}#L{line}">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}: glonk</title>
<meta name="description" content="{{.Description}}">
<meta property="og:type" content="article">
<meta property="og:url" content="https://glonk.dev{{.Address}}">
<meta property="og:title" content="{{.Title}}: glonk">
<meta property="og:description" content="{{.Description}}">
<meta property="og:image" content="https://glonk.dev/card.png">
<meta property="og:image:width" content="1280">
<meta property="og:image:height" content="640">
<meta property="og:image:alt" content="The glonk logo, the words: a terminal rendering engine for Go: 2D, 3D, or both in one frame, and a gold block on a dark floor of lines.">
<meta name="twitter:card" content="summary_large_image">
<link rel="preload" href="/media/fonts/DepartureMono-Regular.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="/style.css">
<link rel="icon" href="/favicon.ico" sizes="32x32">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
</head>
<body>
<!-- Made from the repository by internal/sitepages. Do not edit: change the source and run make site. -->
<a class="skip" href="#text">Skip to the text</a>
<div class="page">

<header class="top">
  <a class="name" href="/"><span class="mark" aria-hidden="true"></span>glonk</a>
  <nav aria-label="glonk">
    <a href="/manual/"{{if eq .Address "/manual/"}} aria-current="page"{{else if .Guide}} aria-current="true"{{end}}>Manual</a>
    <a href="/changelog/"{{if eq .Address "/changelog/"}} aria-current="page"{{end}}>Changelog</a>
    <a href="https://github.com/usrgames/glonk/tree/main/examples">Examples</a>
    <a href="https://github.com/usrgames/glonk">GitHub</a>
  </nav>
</header>

<main{{if .Contents}} class="doc"{{end}}>
<div class="front">
{{.Front}}</div>
{{if .Contents}}
<nav class="contents" aria-label="Contents">
<p>Contents</p>
<ul role="list">
{{range .Contents}}<li><a href="#{{.ID}}">{{.Text}}</a></li>
{{end}}</ul>
</nav>
{{end}}
<article class="text" id="text">
{{.Text}}{{if .Guide}}<nav class="guides" aria-label="Guides">
{{if .Before}}<a rel="prev" href="{{.Before.Address}}">Before: {{.Before.Title}}</a>
{{end}}<a href="/manual/">All guides</a>
{{if .After}}<a rel="next" href="{{.After.Address}}">Next: {{.After.Title}}</a>
{{end}}</nav>
{{end}}</article>
</main>

<footer>
  <nav aria-label="More">
    <a href="https://pkg.go.dev/glonk.dev">pkg.go.dev</a>
    <a href="https://github.com/usrgames/glonk">GitHub</a>
    <a href="/showcase/">Showcase</a>
    <a href="https://github.com/usrgames/glonk/blob/main/CONTRIBUTING.md">Contributing</a>
    <a href="https://github.com/usrgames/glonk/blob/main/SECURITY.md">Security</a>
    <a href="https://github.com/usrgames/glonk/blob/main/BRAND.md">Brand</a>
    <a href="https://github.com/usrgames/glonk/blob/main/LICENSE">MIT license</a>
  </nav>
  <p class="fine">Early and pre-1.0: the API may change. Built to work in most terminals; Windows is untested.</p>
</footer>

</div>
</body>
</html>
`))

// An entry of a contents list, as the shell reads it.
type entry struct{ ID, Text string }

// write gives the HTML of a page.
func (p page) write() ([]byte, error) {
	var contents []entry
	for _, h := range p.Contents {
		if h.level == 2 {
			contents = append(contents, entry{h.id, html.EscapeString(h.text)})
		}
	}
	var out bytes.Buffer
	err := shell.Execute(&out, struct {
		page
		Contents []entry
	}{p, contents})
	return out.Bytes(), err
}

// root is the path of a file of the repository, given from its root.
func root(path string) string { return filepath.Join("..", "..", filepath.FromSlash(path)) }

// source reads a Markdown file of the repository as lines.
func source(path string) ([]line, error) {
	data, err := os.ReadFile(root(path))
	if err != nil {
		return nil, err
	}
	// A checkout on Windows may have turned the ends of the lines into CRLF.
	text := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var lines []line
	for i, t := range strings.Split(text, "\n") {
		lines = append(lines, line{n: i + 1, text: t})
	}
	return lines, nil
}

// sections gives the lines of a file from its first ## heading on, without the sections named.
// What stands above the first ## is given apart. A named section that the file does not have is
// an error: it was renamed, and under its new name it would be in the manual unnoticed.
func sections(lines []line, without []string) (above, kept []line, err error) {
	left := map[string]bool{}
	for _, name := range without {
		left["## "+name] = false
	}
	fenced, started, keep := false, false, false
	for _, l := range lines {
		if strings.HasPrefix(l.text, "```") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(l.text, "## ") {
			_, out := left[l.text]
			if out {
				left[l.text] = true
			}
			started, keep = true, !out
		}
		switch {
		case !started:
			above = append(above, l)
		case keep:
			kept = append(kept, l)
		}
	}
	for name, found := range left {
		if !found {
			return nil, nil, fmt.Errorf("no section %q to leave out: if it was renamed, change the list in internal/sitepages", name)
		}
	}
	return above, kept, nil
}

// link gives the address on the site of what a link in README.md or CHANGELOG.md names.
func link(target string) (string, error) { return linkFrom("")(target) }

// linkFrom gives, for a file in the folder dir of the repository, the address on the site of what
// a link in it names: a section of the page or another site as it is, a file of the site by its
// address there, a guide or the changelog by its page, and any other file of the repository on
// GitHub. A path is
// read from the file's own folder, as GitHub reads it, and the file must be there.
func linkFrom(dir string) func(target string) (string, error) {
	return func(target string) (string, error) {
		if strings.HasPrefix(target, "#") || strings.HasPrefix(target, "https://") {
			return target, nil
		}
		file, _, _ := strings.Cut(target, "#")
		path, part := pathpkg.Join(dir, file), target[len(file):]
		outside := file == "" || strings.HasPrefix(file, "/") || path == ".." || strings.HasPrefix(path, "../")
		if _, err := os.Stat(root(path)); err != nil || outside {
			return "", fmt.Errorf("the link %q is neither https, a section of the page nor a file of the repository", target)
		}
		switch {
		case strings.HasPrefix(path, "site/"):
			return strings.TrimPrefix(path, "site") + part, nil
		case path == "docs/README.md":
			return "/manual/" + part, nil
		case path == "CHANGELOG.md":
			return "/changelog/" + part, nil
		case guideFile.MatchString(path):
			return "/manual/" + guideFile.FindStringSubmatch(path)[1] + "/" + part, nil
		}
		return repository + "/blob/main/" + path + part, nil
	}
}

// A guide's file, from the root of the repository.
var guideFile = regexp.MustCompile(`^docs/([a-z0-9-]+)\.md$`)

// picture gives the HTML of a picture named from the root of the repository.
func picture(alt, src string, linked bool) (string, error) { return pictureFrom("")(alt, src, linked) }

// pictureFrom gives, for a file in the folder dir, the HTML of a picture, which must be a file of
// site/media, with its size so that the page does not jump when it loads. A recording is shown as
// its still, the PNG of the same name, and the still opens the recording: nothing on these pages
// moves by itself.
func pictureFrom(dir string) func(alt, src string, linked bool) (string, error) {
	return func(alt, named string, linked bool) (string, error) {
		src := pathpkg.Join(dir, named)
		if !strings.HasPrefix(src, "site/media/") {
			return "", fmt.Errorf("the picture %q is not a file of site/media", named)
		}
		shown := strings.TrimSuffix(src, ".gif")
		if shown != src {
			shown += ".png"
		}
		f, err := os.Open(root(shown))
		if err != nil {
			return "", fmt.Errorf("the picture %q: %w", named, err)
		}
		defer f.Close()
		size, _, err := image.DecodeConfig(f)
		if err != nil {
			return "", fmt.Errorf("the picture %q: %w", shown, err)
		}
		img := fmt.Sprintf(`<img src="%s" width="%d" height="%d" alt="%s">`,
			strings.TrimPrefix(shown, "site"), size.Width, size.Height, html.EscapeString(alt))
		if shown != src && !linked {
			img = `<a href="` + strings.TrimPrefix(src, "site") + `">` + img + "</a>"
		}
		return img, nil
	}
}

// manual is the page made from docs/README.md, the index of the guides: its title and first
// paragraph above, and the list of guides as its text.
func manual() (page, error) {
	lines, err := source("docs/README.md")
	if err != nil {
		return page{}, err
	}
	// The front is the title and the paragraph under it, to the second blank line.
	split, blanks := len(lines), 0
	for i, l := range lines {
		if l.text == "" {
			if blanks++; blanks == 2 {
				split = i
				break
			}
		}
	}
	r := &renderer{link: linkFrom("docs"), picture: pictureFrom("docs")}
	front, err := r.render(lines[:split])
	if err != nil {
		return page{}, fmt.Errorf("docs/README.md: %w", err)
	}
	text, err := r.render(lines[split:])
	if err != nil {
		return page{}, fmt.Errorf("docs/README.md: %w", err)
	}
	return page{
		Path:        "site/manual/index.html",
		Address:     "/manual/",
		Title:       "Manual",
		Description: "How to use glonk, a terminal rendering engine for Go: guides to a first game, input, testing, pixels, sprites, widgets, 3D, models, a web page and terminals.",
		Front:       strings.Replace(front, "<p>", `<p class="lede">`, 1),
		Text:        text,
	}, nil
}

// changelog is the page made from CHANGELOG.md: all of it, with its title and what stands under
// it above the contents.
func changelog() (page, error) {
	lines, err := source("CHANGELOG.md")
	if err != nil {
		return page{}, err
	}
	above, kept, err := sections(lines, nil)
	if err != nil {
		return page{}, fmt.Errorf("CHANGELOG.md: %w", err)
	}
	r := &renderer{link: link, picture: picture}
	// The targets of the version links are at the foot of the file, so the text is read first.
	text, err := r.render(kept)
	if err != nil {
		return page{}, fmt.Errorf("CHANGELOG.md: %w", err)
	}
	contents := r.headings
	front, err := r.render(above)
	if err != nil {
		return page{}, fmt.Errorf("CHANGELOG.md: %w", err)
	}
	return page{
		Path:        "site/changelog/index.html",
		Address:     "/changelog/",
		Title:       "Changelog",
		Description: "What each release of glonk added, changed and fixed.",
		Front:       strings.Replace(front, "<p>", `<p class="lede">`, 1),
		Contents:    contents,
		Text:        text,
	}, nil
}

// A link in docs/README.md to a guide, a file beside it.
var guideLink = regexp.MustCompile(`\]\(([a-z0-9-]+)\.md(?:#[^)]*)?\)`)

// guideNames gives the guides in the order docs/README.md, their index, first links each. The
// index and the folder must agree: a guide the index leaves out would have a page that nothing
// leads to, and a link to a file that is not there leads nowhere.
func guideNames() ([]string, error) {
	index, err := source("docs/README.md")
	if err != nil {
		return nil, err
	}
	var names []string
	listed, fenced := map[string]bool{}, false
	for _, l := range index {
		if strings.HasPrefix(l.text, "```") {
			fenced = !fenced
		}
		if fenced {
			continue
		}
		for _, m := range guideLink.FindAllStringSubmatch(l.text, -1) {
			if _, err := os.Stat(root("docs/" + m[1] + ".md")); err != nil {
				return nil, fmt.Errorf("docs/README.md line %d: it links %s.md, which is not in docs", l.n, m[1])
			}
			if !listed[m[1]] {
				listed[m[1]] = true
				names = append(names, m[1])
			}
		}
	}
	files, err := filepath.Glob(root("docs/*.md"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		switch {
		case name == "README":
		case !guideFile.MatchString("docs/" + name + ".md"):
			return nil, fmt.Errorf("docs/%s.md: a guide's name is lower-case letters, digits and hyphens, since it is the page's address", name)
		case !listed[name]:
			return nil, fmt.Errorf("docs/%s.md is not linked from docs/README.md, which gives the guides their order", name)
		}
	}
	return names, nil
}

// guide is the page made from a file of docs: its title and what stands under it above the
// contents, then its sections. The page's description is its first paragraph.
func guide(name string) (page, error) {
	path := "docs/" + name + ".md"
	lines, err := source(path)
	if err != nil {
		return page{}, err
	}
	above, kept, err := sections(lines, nil)
	if err != nil {
		return page{}, fmt.Errorf("%s: %w", path, err)
	}
	r := &renderer{link: linkFrom("docs"), picture: pictureFrom("docs")}
	text, err := r.render(kept)
	if err != nil {
		return page{}, fmt.Errorf("%s: %w", path, err)
	}
	contents := r.headings
	front, err := r.render(above)
	if err != nil {
		return page{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(above) == 0 || !strings.HasPrefix(above[0].text, "# ") || len(contents) == 0 {
		return page{}, fmt.Errorf("%s: a guide opens with its title, a line that starts with one #, and has sections under ## headings", path)
	}
	// What the title is followed by, to its first blank line, is what the guide is about.
	var first []string
	for _, l := range above[1:] {
		if l.text == "" && len(first) > 0 {
			break
		}
		if l.text != "" {
			first = append(first, l.text)
		}
	}
	_, about, err := r.inline(above[0].n, strings.Join(first, " "))
	if err != nil {
		return page{}, fmt.Errorf("%s: %w", path, err)
	}
	if about == "" {
		return page{}, fmt.Errorf("%s: a guide says what it is about in a paragraph under its title", path)
	}
	return page{
		Path:        "site/manual/" + name + "/index.html",
		Address:     "/manual/" + name + "/",
		Title:       html.EscapeString(r.headings[len(contents)].text),
		Description: html.EscapeString(about),
		Front:       strings.Replace(front, "<p>", `<p class="lede">`, 1),
		Contents:    contents,
		Text:        text,
		Guide:       true,
	}, nil
}

// guides gives the page of every guide, each with the one before it and the one after it.
func guides() ([]page, error) {
	names, err := guideNames()
	if err != nil {
		return nil, err
	}
	var pages []page
	for _, name := range names {
		p, err := guide(name)
		if err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	for i := range pages {
		if i > 0 {
			pages[i].Before = &neighbor{pages[i-1].Address, pages[i-1].Title}
		}
		if i < len(pages)-1 {
			pages[i].After = &neighbor{pages[i+1].Address, pages[i+1].Title}
		}
	}
	return pages, nil
}

// pages gives every page this package makes.
func pages() ([]page, error) {
	var all []page
	for _, build := range []func() (page, error){manual, changelog, showcase} {
		p, err := build()
		if err != nil {
			return nil, err
		}
		all = append(all, p)
	}
	g, err := guides()
	return append(all, g...), err
}

// The pages are what the guides and CHANGELOG.md say now. Run with -update, or make site, to
// write them again.
func TestThePagesAreWhatTheRepositorySays(t *testing.T) {
	// The pictures are a module of their own, so that a download of glonk leaves them out (see
	// site/media/go.mod), and a page cannot be made without the sizes of its pictures.
	if _, err := os.Stat(root("site/media")); os.IsNotExist(err) {
		t.Skip("site/media is not in this copy of glonk")
	}
	all, err := pages()
	if err != nil {
		t.Fatal(err)
	}
	made := map[string]bool{}
	for _, p := range all {
		made[p.Path] = true
		want, err := p.write()
		if err != nil {
			t.Fatal(err)
		}
		if *update {
			if err := os.MkdirAll(filepath.Dir(root(p.Path)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(root(p.Path), want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := os.ReadFile(root(p.Path))
		if err != nil {
			t.Fatalf("wanted %s (run make site to write it): %v", p.Path, err)
		}
		got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
		if !bytes.Equal(got, want) {
			t.Errorf("%s is not what the repository says now: run make site to write it again.\n%s", p.Path, firstDifference(got, want))
		}
	}
	// A guide that was renamed or taken away leaves its page behind, which nothing would notice.
	old, err := filepath.Glob(root("site/manual/*/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range old {
		if path := "site/manual/" + filepath.Base(filepath.Dir(f)) + "/index.html"; !made[path] {
			t.Errorf("%s is the page of no guide in docs: delete its folder", path)
		}
	}
}

// firstDifference shows where two texts part, with a little of what stands before.
func firstDifference(got, want []byte) string {
	at := 0
	for at < len(got) && at < len(want) && got[at] == want[at] {
		at++
	}
	window := func(b []byte) []byte { return b[max(0, at-40):min(len(b), at+40)] }
	return fmt.Sprintf("they part at byte %d:\ngot  %q\nwant %q", at, window(got), window(want))
}
