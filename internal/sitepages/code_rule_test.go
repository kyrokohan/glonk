package sitepages

import (
	"strings"
	"testing"
)

func TestGoBlocks(t *testing.T) {
	got := goBlocks(md("Text.\n\n```go\n\tcam := mesh.Camera{}\n\n\n\tball.Draw(fb, cam)\n\tfb.Clear()\n```\n\n```sh\ngo run .\n```\n\n```go\n```"))
	if len(got) != 2 || got[0].line != 3 || len(got[0].parts) != 2 || got[0].parts[0][0] != "cam := mesh.Camera{}" ||
		strings.Join(got[0].parts[1], "|") != "ball.Draw(fb, cam)|fb.Clear()" || got[1].line != 15 || len(got[1].parts) != 0 {
		t.Errorf("wanted the two blocks of Go, the first in two parts without indentation and the second empty, got %+v", got)
	}
}

func TestHeld(t *testing.T) {
	file := []string{"package main", "", "func main() {", "a := 1", "b := 2", "", "use(a, b)", "}"}
	for _, c := range []struct {
		name  string
		parts [][]string
		want  int
	}{
		{"lines one after another", [][]string{{"a := 1", "b := 2"}}, 1},
		{"parts in order, with lines left out between them", [][]string{{"func main() {"}, {"b := 2"}, {"use(a, b)", "}"}}, 3},
		{"a part the file does not have", [][]string{{"a := 1"}, {"c := 3"}}, 1},
		{"a line changed", [][]string{{"a := 1", "b := 3"}}, 0},
		{"lines left out inside a part", [][]string{{"a := 1", "use(a, b)"}}, 0},
		{"parts out of order", [][]string{{"b := 2"}, {"a := 1"}}, 1},
		{"a part twice, when the file has it once", [][]string{{"a := 1"}, {"a := 1"}}, 1},
		{"a part that runs past the end", [][]string{{"}", "}"}}, 0},
	} {
		if got := held(file, c.parts); got != c.want {
			t.Errorf("%s: wanted %d of its parts held, got %d", c.name, c.want, got)
		}
	}
}

// A block that no file holds is named with its line and the part that is missing.
func TestUncompiledNamesTheBlockAndThePart(t *testing.T) {
	files := map[string][]string{"a.go": {"a := 1", "b := 2"}, "b.go": {"a := 1", "", "c := 3"}}
	lines := md("```go\na := 1\n\nc := 3\n```\n\n```go\na := 1\n\nd := 4\n```\n\n```go\n```")
	got := uncompiled("docs/x.md", lines, files)
	if len(got) != 2 || !strings.HasPrefix(got[0], "docs/x.md:7: ") || !strings.Contains(got[0], `"d := 4"`) || !strings.HasPrefix(got[1], "docs/x.md:13: ") {
		t.Errorf("wanted the second block refused for its part d := 4 and the third for being empty, got %q", got)
	}
}

func TestTooLong(t *testing.T) {
	long := strings.Repeat("é", longestParagraph+1)
	lines := md(strings.Repeat("é", longestParagraph) + "\n\n- " + long + "\n\n| " + long + " |\n\n```\n" + long + "\n```\n\n" + long)
	got := tooLong(lines)
	if len(got) != 2 || got[0].n != 3 || got[1].n != 11 {
		t.Errorf("wanted the bullet and the last paragraph, counted in characters, and neither the table nor the code, got %+v", got)
	}
}
