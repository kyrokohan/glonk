// Package sitecheck holds the tests of the website in ../../site. It has no code of
// its own: it sits under internal so that it stays out of the documentation and out of
// the folder Cloudflare serves.
package sitecheck

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// What the go command must read from every page. Its answer to glonk.dev/engine?go-get=1
// is the not-found page, because the command ignores the status code.
const (
	wantImport = "glonk.dev git https://github.com/kyrokohan/glonk"
	wantSource = "glonk.dev https://github.com/kyrokohan/glonk https://github.com/kyrokohan/glonk/tree/main{/dir} https://github.com/kyrokohan/glonk/blob/main{/dir}/{file}#L{line}"
)

// pages are the HTML files of the site, by their paths in site/: the two written by hand and
// those internal/sitepages makes. The folder of recordings and fonts has none.
var pages = func() []string {
	var found []string
	site := filepath.Join("..", "..", "site")
	err := filepath.WalkDir(site, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "media":
			return filepath.SkipDir
		case !d.IsDir() && filepath.Ext(path) == ".html":
			rel, err := filepath.Rel(site, path)
			if err != nil {
				return err
			}
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return found
}()

// The tests below go over every page that is found. This one fails if a page was not.
func TestTheSiteHasItsPages(t *testing.T) {
	for _, want := range []string{"index.html", "404.html", "manual/index.html", "changelog/index.html", "showcase/index.html"} {
		if !slices.Contains(pages, want) {
			t.Errorf("wanted %s among the pages of the site, got %v", want, pages)
		}
	}
}

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
// Cloudflare builds every pull request branch with `wrangler preview`, which stops with
// an error unless the file has a previews block, even an empty one. The file must stay
// plain JSON, with no comments, for this test to read it.
func TestCloudflareServesTheSiteFolderWithItsNotFoundPage(t *testing.T) {
	var cfg struct {
		Name   string `json:"name"`
		Assets struct {
			Directory string `json:"directory"`
			NotFound  string `json:"not_found_handling"`
		} `json:"assets"`
		Previews map[string]any `json:"previews"`
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
	if cfg.Previews == nil {
		t.Errorf(`wanted a "previews" block (it may be empty), without which Cloudflare's builds of pull request branches fail`)
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
		// One page has one script, for the game of the cover; see TestTheOneScriptIsTheGame.
		scripts := strings.Count(strings.ToLower(page), "<script")
		if want := map[bool]int{true: 1, false: 0}[name == "index.html"]; scripts != want {
			t.Errorf("%s: wanted %d script, found %d", name, want, scripts)
		}
		// Cloudflare rewrites email addresses it finds in a page.
		if m := email.FindString(page); m != "" {
			t.Errorf("%s: wanted no email address, found %q", name, m)
		}
	}
}

// Every page names the three icons, which internal/brand draws: one for browsers that take an
// SVG, one for those that do not, and one for a phone's home screen.
func TestEveryPageLinksTheIcons(t *testing.T) {
	for _, name := range pages {
		page := read(t, "site", name)
		for _, link := range []string{
			`<link rel="icon" href="/favicon.ico" sizes="32x32">`,
			`<link rel="icon" href="/favicon.svg" type="image/svg+xml">`,
			`<link rel="apple-touch-icon" href="/apple-touch-icon.png">`,
		} {
			if !strings.Contains(page, link) {
				t.Errorf("%s: wanted %s", name, link)
			}
		}
	}
}

// What a chat or a social site shows beside a link to glonk.dev comes from these tags. The picture
// is named by its full address, which the test for local links does not follow, so it is checked
// here that the address is a file of the site.
func TestTheLinkPreviewNamesAFileOfTheSite(t *testing.T) {
	index := read(t, "site", "index.html")
	for _, tag := range []string{`<meta property="og:title" content="`, `<meta property="og:description" content="`, `<meta property="og:image:alt" content="`, `<meta name="twitter:card" content="summary_large_image">`} {
		if !strings.Contains(index, tag) {
			t.Errorf("index.html: wanted %s", tag)
		}
	}
	m := regexp.MustCompile(`<meta property="og:image" content="https://glonk\.dev/([^"]+)">`).FindStringSubmatch(index)
	if m == nil {
		t.Fatal(`index.html: wanted <meta property="og:image"> with an address on glonk.dev`)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "site", m[1])); err != nil {
		t.Errorf("index.html: wanted the picture of the link preview, %q, to be a file of the site, got %v", m[1], err)
	}
}

// The site was without a script until the game of the cover (#104). The rule since: only the
// landing page loads one, it is /play.js, a file of the site, and nothing is written inside the
// page, so that what runs is one file anyone can read. The files the script loads are files of
// the site too, or the ones make play writes before each deploy, which are not committed. Since
// #148 one of those is glonk.js, the part of the game any site can copy from the web package:
// play.js loads it as it loads xterm.js, so the page still has one script tag.
func TestTheOneScriptIsTheGame(t *testing.T) {
	const tag = `<script src="/play.js" defer></script>`
	if index := read(t, "site", "index.html"); strings.Count(index, tag) != 1 {
		t.Errorf("index.html: wanted its one script to be %s", tag)
	}

	built := map[string]bool{"/media/play/cube3d.wasm.gz": true, "/media/play/wasm_exec.js": true, "/media/play/glonk.js": true}
	makefile := read(t, "Makefile")
	for _, want := range []string{"-o $(PLAY)/cube3d.wasm ", "gzip -9nf $(PLAY)/cube3d.wasm\n", "lib/wasm/wasm_exec.js\" $(PLAY)/", "cp web/glonk.js $(PLAY)/\n"} {
		if !strings.Contains(makefile, want) {
			t.Errorf("Makefile: wanted make play to write what the script loads, with %q", want)
		}
	}

	script := read(t, "site", "play.js")
	named := regexp.MustCompile(`"(/[^"]*|[a-z]+://[^"]*)"`).FindAllStringSubmatch(script, -1)
	if len(named) < 5 {
		t.Fatalf("play.js: wanted the files it loads, found %v", named)
	}
	for _, m := range named {
		switch path := m[1]; {
		case !strings.HasPrefix(path, "/media/"):
			t.Errorf("play.js: wanted %q to be a file of the site in /media/", path)
		case built[path], noMedia():
		default:
			if _, err := os.Stat(filepath.Join("..", "..", "site", path)); err != nil {
				t.Errorf("play.js: wanted %q to be a file of the site, got %v", path, err)
			}
		}
	}
}

// noMedia reports whether site/media is absent. The recordings and the fonts are a module of their
// own, so that a download of glonk leaves them out (see site/media/go.mod). A copy of glonk in the
// module cache therefore has no such folder, and only there are links into it let through.
func noMedia() bool {
	_, err := os.Stat(filepath.Join("..", "..", "site", "media"))
	return os.IsNotExist(err)
}

// The pages ask no other site for anything: the fonts, the stylesheet and the pictures are files
// of the site. What a page loads is what its link tags, its pictures and its stylesheet name; a
// link to follow is not loaded. On glonk.dev Cloudflare adds one script at the edge, its Web
// Analytics beacon, which is a setting of the account and in no file here: it was found on
// 2026-11-19 and is kept, so every page view on glonk.dev also goes to Cloudflare. Previews and
// this test do not see it.
func TestNothingIsLoadedFromAnotherSite(t *testing.T) {
	local := func(target string) bool {
		return strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//")
	}
	loads := regexp.MustCompile(`<link\b[^>]*\bhref="([^"]*)"|\bsrc(?:set)?="([^"]*)"`)
	for _, name := range pages {
		for _, m := range loads.FindAllStringSubmatch(read(t, "site", name), -1) {
			if target := m[1] + m[2]; !local(target) {
				t.Errorf("%s: wanted %q to be loaded from this site", name, target)
			}
		}
	}

	css := read(t, "site", "style.css")
	if strings.Contains(css, "@import") {
		t.Error("style.css: wanted no @import")
	}
	urls := regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)`).FindAllStringSubmatch(css, -1)
	if len(urls) == 0 {
		t.Error("style.css: wanted it to name the fonts of BRAND.md")
	}
	for _, m := range urls {
		if !local(m[1]) {
			t.Errorf("style.css: wanted %q to be loaded from this site", m[1])
			continue
		}
		if noMedia() && strings.HasPrefix(m[1], "/media/") {
			continue
		}
		if _, err := os.Stat(filepath.Join("..", "..", "site", m[1])); err != nil {
			t.Errorf("style.css: wanted %q to be a file of the site, got %v", m[1], err)
		}
	}
}

// A link to this site must name a file that is in it, and a link to a part of a page, on this
// page or another, an id that the page has. The manual's links between its sections come from
// the README, where they are made for GitHub's ids.
func TestLocalLinksNameFilesOfTheSite(t *testing.T) {
	link := regexp.MustCompile(`(?:href|src|srcset)="([^"]*)"`)
	noRecordings := noMedia()
	for _, name := range pages {
		page := read(t, "site", name)

		found := 0
		for _, m := range link.FindAllStringSubmatch(page, -1) {
			if regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`).MatchString(m[1]) {
				continue // another site
			}
			target, part, _ := strings.Cut(m[1], "#")
			target, _, _ = strings.Cut(target, "?")
			in := page
			if target != "" {
				found++
				if strings.HasSuffix(target, "/") {
					target += "index.html"
				}
				if noRecordings && strings.HasPrefix(target, "/media/") {
					continue
				}
				data, err := os.ReadFile(filepath.Join("..", "..", "site", strings.TrimPrefix(target, "/")))
				if err != nil {
					t.Errorf("%s: wanted %q to be a file of the site, got %v", name, m[1], err)
					continue
				}
				in = string(data)
			}
			if part != "" && !strings.Contains(in, `id="`+part+`"`) {
				t.Errorf("%s: wanted %q to name an id of the page it leads to", name, m[1])
			}
		}
		if found == 0 {
			t.Errorf("%s: wanted at least the stylesheet among local links", name)
		}

		// Of two parts with one id, a link reaches the first only.
		seen := map[string]bool{}
		for _, m := range regexp.MustCompile(` id="([^"]*)"`).FindAllStringSubmatch(page, -1) {
			if seen[m[1]] {
				t.Errorf("%s: wanted the id %q once", name, m[1])
			}
			seen[m[1]] = true
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
