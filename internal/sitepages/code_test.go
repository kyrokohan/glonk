package sitepages

import (
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// The longest a paragraph or a bullet of a guide may be, in characters: one idea to each.
const longestParagraph = 400

// checkedDocs gives the Markdown files whose Go and whose paragraphs are held to the rules below:
// the README, the guides and their index, and the examples' README.
func checkedDocs() ([]string, error) {
	files, err := filepath.Glob(root("docs/*.md"))
	for i, f := range files {
		files[i] = "docs/" + filepath.Base(f)
	}
	return append([]string{"README.md", "examples/README.md"}, files...), err
}

// A block of Go in a Markdown file: what stands between ```go and ```.
type goBlock struct {
	line  int        // of the ```go
	parts [][]string // its lines without their indentation, parted where a line is blank
}

// goBlocks gives the blocks of Go in lines of Markdown.
func goBlocks(lines []line) []goBlock {
	var blocks []goBlock
	for i := 0; i < len(lines); i++ {
		if lines[i].text != "```go" {
			continue
		}
		b := goBlock{line: lines[i].n}
		var part []string
		for i++; i < len(lines) && lines[i].text != "```"; i++ {
			text := strings.TrimSpace(lines[i].text)
			if text != "" {
				part = append(part, text)
				continue
			}
			if part != nil {
				b.parts, part = append(b.parts, part), nil
			}
		}
		if part != nil {
			b.parts = append(b.parts, part)
		}
		blocks = append(blocks, b)
	}
	return blocks
}

// held counts the parts, from the first on, that a file has, each as lines one after another and
// each after the part before it.
func held(file []string, parts [][]string) int {
	from := 0
	for n, part := range parts {
		at := -1
	search:
		for i := from; i+len(part) <= len(file); i++ {
			for j, want := range part {
				if file[i+j] != want {
					continue search
				}
			}
			at = i
			break
		}
		if at < 0 {
			return n
		}
		from = at + len(part)
	}
	return len(parts)
}

// goFiles gives every Go file of the repository as its lines without their indentation.
func goFiles() (map[string][]string, error) {
	files := map[string][]string{}
	err := filepath.WalkDir(root("."), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// The site holds no Go, and a folder whose name starts with a dot is not the repository's.
			if name := d.Name(); name == "site" || (strings.HasPrefix(name, ".") && len(name) > 2) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var lines []string
		for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			lines = append(lines, strings.TrimSpace(l))
		}
		files[path] = lines
		return nil
	})
	return files, err
}

// uncompiled gives, for each block of Go in a Markdown file that no Go file holds, what is wrong
// with it.
func uncompiled(path string, lines []line, files map[string][]string) []string {
	var wrong []string
	for _, b := range goBlocks(lines) {
		if len(b.parts) == 0 {
			wrong = append(wrong, fmt.Sprintf("%s:%d: a block of Go with nothing in it", path, b.line))
			continue
		}
		most := 0
		for _, file := range files {
			most = max(most, held(file, b.parts))
		}
		if most < len(b.parts) {
			wrong = append(wrong, fmt.Sprintf("%s:%d: this Go is in no Go file of the repository, so nothing compiles it. No file has its part that starts %q, after the parts above it. Copy the code from a file that is built: an example, or an Example test.",
				path, b.line, clip(b.parts[most][0])))
		}
	}
	return wrong
}

// The Go in the guides compiles and runs, because it is not written there: every block is lines
// of a Go file of this repository, which go vet builds and, where it is an Example, go test runs.
// A block may leave lines out between its parts, where it has a blank line, and never inside one.
func TestTheGoInTheGuidesIsInFilesThatCompile(t *testing.T) {
	files, err := goFiles()
	if err != nil {
		t.Fatal(err)
	}
	docs, err := checkedDocs()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range docs {
		lines, err := source(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range uncompiled(path, lines, files) {
			t.Error(w)
		}
	}
}

// A block of code on a page of the site.
var preCode = regexp.MustCompile(`(?s)<pre[^>]*><code>(.*?)</code></pre>`)

// pageBlocks gives the blocks of code on a page of the site that is written by hand, as the
// lines of Markdown a block of Go would be, with its line in the page.
func pageBlocks(page string) []goBlock {
	var blocks []goBlock
	for _, m := range preCode.FindAllStringSubmatchIndex(page, -1) {
		code := html.UnescapeString(page[m[2]:m[3]])
		lines := md("```go\n" + code + "\n```")
		b := goBlocks(lines)[0]
		b.line = 1 + strings.Count(page[:m[0]], "\n")
		blocks = append(blocks, b)
	}
	return blocks
}

// The code on the cover of glonk.dev runs too: its blocks are lines of Go files, as a guide's
// are. The cover is written by hand, so this is what keeps its six steps and its program true.
func TestTheCodeOnTheCoverIsInFilesThatCompile(t *testing.T) {
	files, err := goFiles()
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(root("site/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	blocks := pageBlocks(string(page))
	if len(blocks) == 0 {
		t.Fatal("wanted the code of the cover, found no <pre><code> in site/index.html")
	}
	for _, b := range blocks {
		most := 0
		for _, file := range files {
			most = max(most, held(file, b.parts))
		}
		if most < len(b.parts) {
			t.Errorf("site/index.html:%d: this code is in no Go file of the repository, so nothing compiles it. No file has its part that starts %q.", b.line, clip(b.parts[most][0]))
		}
	}
}

// tooLong gives the lines of Markdown that are a paragraph or a bullet of more than the longest
// allowed. Code and the rows of a table are not counted.
func tooLong(lines []line) []line {
	var long []line
	fenced := false
	for _, l := range lines {
		if strings.HasPrefix(l.text, "```") {
			fenced = !fenced
			continue
		}
		if !fenced && !strings.HasPrefix(l.text, "|") && utf8.RuneCountInString(l.text) > longestParagraph {
			long = append(long, l)
		}
	}
	return long
}

// A guide is read through, so it says one thing in a paragraph or a bullet and then stops. The
// README that the guides replace had bullets of 1,500 characters.
func TestTheGuidesKeepTheirParagraphsShort(t *testing.T) {
	docs, err := checkedDocs()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range docs {
		lines, err := source(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range tooLong(lines) {
			t.Errorf("%s:%d: %d characters in one paragraph or bullet, and %d is the most: say one thing, then start another",
				path, l.n, utf8.RuneCountInString(l.text), longestParagraph)
		}
	}
}
