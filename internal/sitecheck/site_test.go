// Package sitecheck holds the tests of the website in ../../site. It has no code of
// its own: it sits under internal so that it stays out of the documentation and out of
// the folder Cloudflare Pages serves.
package sitecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// What the go command must read from every page. Its answer to glonk.dev/engine?go-get=1
// is the not-found page, because the command ignores the status code.
const (
	wantImport = "glonk.dev git https://github.com/kyrokohan/glonk"
	wantSource = "glonk.dev https://github.com/kyrokohan/glonk https://github.com/kyrokohan/glonk/tree/main{/dir} https://github.com/kyrokohan/glonk/blob/main{/dir}/{file}#L{line}"
)

var pages = []string{"index.html", "404.html"}

// read returns the text of a file of the repository, given its path from the root.
func read(t *testing.T, path ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, path...)...))
	if err != nil {
		t.Fatalf("unable to read %s: %v", filepath.Join(path...), err)
	}
	return string(b)
}

// metas returns the content of every <meta name="name"> tag of a page.
func metas(page, name string) []string {
	re := regexp.MustCompile(`<meta\s+name="` + regexp.QuoteMeta(name) + `"\s+content="([^"]*)"\s*>`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestEveryPageCarriesTheGoImportAndGoSourceTags(t *testing.T) {
	for _, name := range pages {
		page := read(t, "site", name)

		if got := metas(page, "go-import"); len(got) != 1 || got[0] != wantImport {
			t.Errorf("%s: wanted one go-import tag with %q, got %q", name, wantImport, got)
		}
		if got := metas(page, "go-source"); len(got) != 1 || got[0] != wantSource {
			t.Errorf("%s: wanted one go-source tag with %q, got %q", name, wantSource, got)
		}
	}
}

// The go command reads the head with a restricted parser and the reference asks for the
// tag early, before any CSS or script.
func TestTheTagsComeBeforeAnythingTheParserMightStopAt(t *testing.T) {
	for _, name := range pages {
		page := read(t, "site", name)

		limit := len(page)
		for _, marker := range []string{"<link", "<style", "<script", "<body"} {
			if i := strings.Index(page, marker); i >= 0 {
				limit = min(limit, i)
			}
		}
		for _, tag := range []string{`name="go-import"`, `name="go-source"`} {
			i := strings.Index(page, tag)
			if i < 0 || i > limit {
				t.Errorf("%s: wanted %s before the first link, style, script or body (at %d), found it at %d", name, tag, limit, i)
			}
		}
	}
}

// wrangler.jsonc tells Cloudflare which folder to serve and what to send for a path with
// no file. Without the 404 page a request for glonk.dev/engine?go-get=1 would get an
// empty 404 with no tags, so the go command could not find the module from a sub-path.
// The file must stay plain JSON, with no comments, for this test to read it.
func TestCloudflareServesTheSiteFolderWithItsNotFoundPage(t *testing.T) {
	var cfg struct {
		Name   string `json:"name"`
		Assets struct {
			Directory string `json:"directory"`
			NotFound  string `json:"not_found_handling"`
		} `json:"assets"`
	}
	if err := json.Unmarshal([]byte(read(t, "wrangler.jsonc")), &cfg); err != nil {
		t.Fatalf("wanted wrangler.jsonc to be plain JSON, got %v", err)
	}

	if cfg.Name == "" {
		t.Errorf("wanted the Worker to have a name")
	}
	if got := filepath.Clean(cfg.Assets.Directory); got != "site" {
		t.Errorf("wanted the served directory to be site, got %q", cfg.Assets.Directory)
	}
	if cfg.Assets.NotFound != "404-page" {
		t.Errorf(`wanted not_found_handling "404-page", so unknown paths get 404.html, got %q`, cfg.Assets.NotFound)
	}
}

func TestPagesAreSelfContainedAndPlain(t *testing.T) {
	email := regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	for _, name := range pages {
		page := read(t, "site", name)

		for _, want := range []string{`<html lang="`, `<meta name="viewport"`, `<meta charset="utf-8">`} {
			if !strings.Contains(page, want) {
				t.Errorf("%s: wanted %s", name, want)
			}
		}
		if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(page); m == nil || strings.TrimSpace(m[1]) == "" {
			t.Errorf("%s: wanted a title", name)
		}
		if strings.Contains(strings.ToLower(page), "<script") {
			t.Errorf("%s: wanted no script", name)
		}
		// Cloudflare rewrites email addresses it finds in a page.
		if m := email.FindString(page); m != "" {
			t.Errorf("%s: wanted no email address, found %q", name, m)
		}
	}
}

// A link to this site must name a file that is in it.
func TestLocalLinksNameFilesOfTheSite(t *testing.T) {
	link := regexp.MustCompile(`(?:href|src)="([^"]*)"`)
	for _, name := range pages {
		page := read(t, "site", name)

		found := 0
		for _, m := range link.FindAllStringSubmatch(page, -1) {
			target := m[1]
			if strings.HasPrefix(target, "#") || regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`).MatchString(target) {
				continue // inside the page, or another site
			}
			found++
			target, _, _ = strings.Cut(target, "#")
			target, _, _ = strings.Cut(target, "?")
			if target == "/" || target == "" {
				target = "index.html"
			}
			if _, err := os.Stat(filepath.Join("..", "..", "site", strings.TrimPrefix(target, "/"))); err != nil {
				t.Errorf("%s: wanted %q to be a file of the site, got %v", name, m[1], err)
			}
		}
		if found == 0 {
			t.Errorf("%s: wanted at least the stylesheet among local links", name)
		}
	}
}

// What a person is told to type must be what works: the module path in go.mod, the
// install line of the README and of the page, and the documentation link all agree. The
// rename to glonk.dev changes all of them together, and this fails if one is left behind.
func TestInstallLineAgreesWithTheReadmeAndGoMod(t *testing.T) {
	module := regexp.MustCompile(`(?m)^module (\S+)$`).FindStringSubmatch(read(t, "go.mod"))
	readme := regexp.MustCompile(`(?m)^go get (\S+)$`).FindStringSubmatch(read(t, "README.md"))
	index := read(t, "site", "index.html")
	onPage := regexp.MustCompile(`<code>go get ([^<\s]+)</code>`).FindStringSubmatch(index)
	if module == nil || readme == nil || onPage == nil {
		t.Fatalf("wanted a module line, a README install line and a page install line, got %q, %q, %q", module, readme, onPage)
	}

	if readme[1] != module[1] {
		t.Errorf("wanted the README to install %q (the module in go.mod), got %q", module[1], readme[1])
	}
	if onPage[1] != module[1] {
		t.Errorf("wanted the page to install %q (the module in go.mod), got %q", module[1], onPage[1])
	}
	if want := `href="https://pkg.go.dev/` + module[1] + `"`; !strings.Contains(index, want) {
		t.Errorf("wanted the documentation link %s on the page", want)
	}
}
