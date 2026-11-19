package sitepages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

// A project made with glonk, as showcase.json lists it. Everything in it was written by somebody
// else, so it is checked before it is shown, and html/template makes it safe in the page.
type project struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Line       string `json:"line"`      // one sentence on what it is
	Picture    string `json:"picture"`   // a PNG in site/media/showcase
	Recording  string `json:"recording"` // a GIF beside it of the same name, or nothing
	Alt        string `json:"alt"`       // what the picture shows
	Glonk      string `json:"glonk"`     // the release it builds with
	License    string `json:"license"`
}

// The rule for being listed. The page states it, and the form, in
// .github/ISSUE_TEMPLATE/show_your_project.yml, asks for each line of it to be ticked: a test
// holds the form to these words.
var (
	listingRule = []string{
		"The repository is public and under an open source license.",
		"It builds against a released version of glonk.",
		"Its README says how to run it.",
		"Its author agrees to the listing, and to its picture being shown on glonk.dev.",
	}
	maintainerMay = "The maintainer may decline or remove a listing."
)

const (
	showcaseList = "internal/sitepages/showcase.json"
	showcaseForm = ".github/ISSUE_TEMPLATE/show_your_project.yml"
)

// Where the pictures of the showcase are, and how large one may be. A listed picture is a copy in
// this repository, so that a listing cannot change after it is accepted, and the folder is in
// site/media, which a download of glonk leaves out. They are variables for the tests.
var (
	showcaseFolder = "site/media/showcase/"
	maxPicture     = int64(300 << 10)
	maxRecording   = int64(2 << 20)
)

// projects reads the list. A field it does not know is an error, as a misspelt one would be.
func projects(data []byte) ([]project, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var list []project
	if err := dec.Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

// check says what is wrong with an entry of the list, given the releases of glonk there are.
func (p project) check(releases map[string]bool) error {
	switch u, err := url.Parse(p.Repository); {
	case strings.TrimSpace(p.Name) == "":
		return fmt.Errorf("an entry has no name")
	case err != nil || u.Scheme != "https" || u.Host == "":
		return fmt.Errorf("%s: the repository %q is not an https address", p.Name, p.Repository)
	case strings.TrimSpace(p.Line) == "" || strings.Contains(p.Line, "\n") || len([]rune(p.Line)) > 140:
		return fmt.Errorf("%s: the line on what it is must be one line of at most 140 characters", p.Name)
	case strings.TrimSpace(p.Alt) == "":
		return fmt.Errorf("%s: the picture needs words for what it shows", p.Name)
	case strings.TrimSpace(p.License) == "":
		return fmt.Errorf("%s: no license is named", p.Name)
	case !releases[p.Glonk]:
		return fmt.Errorf("%s: %q is not a release of glonk that CHANGELOG.md names", p.Name, p.Glonk)
	}
	if err := pictureFile(p.Picture, ".png", "png", maxPicture); err != nil {
		return fmt.Errorf("%s: %w", p.Name, err)
	}
	if p.Recording == "" {
		return nil
	}
	if p.Recording != strings.TrimSuffix(p.Picture, ".png")+".gif" {
		return fmt.Errorf("%s: the recording must have the picture's name, ending in .gif", p.Name)
	}
	if err := pictureFile(p.Recording, ".gif", "gif", maxRecording); err != nil {
		return fmt.Errorf("%s: %w", p.Name, err)
	}
	return nil
}

// pictureFile checks that a file of the showcase is in its folder, is the kind of picture its
// name says and is not larger than it may be.
func pictureFile(name, ext, format string, limit int64) error {
	if !strings.HasPrefix(name, showcaseFolder) || path.Ext(name) != ext || strings.Contains(name, "..") {
		return fmt.Errorf("%q is not a %s file in %s", name, ext, showcaseFolder)
	}
	return pictureIs(root(name), format, limit)
}

// pictureIs checks what is in a file, whatever it is called: a recording named .png would be
// shown as a still, and move.
func pictureIs(file, format string, limit int64) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > limit {
		return fmt.Errorf("%q is %d KB, and may be %d at most", file, info.Size()>>10, limit>>10)
	}
	if _, got, err := image.DecodeConfig(f); err != nil || got != format {
		return fmt.Errorf("%q is not a %s picture (%s, %v)", file, format, got, err)
	}
	return nil
}

// releases gives the versions of glonk that CHANGELOG.md has a section for, as v0.12.0.
func releases() (map[string]bool, error) {
	data, err := os.ReadFile(root("CHANGELOG.md"))
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]`).FindAllStringSubmatch(string(data), -1) {
		found["v"+m[1]] = true
	}
	return found, nil
}

// What the showcase says under its title: the projects, where there are some, then how to be
// listed and the badge. Before the first project it is an invitation, and does not say that the
// list is empty.
var showcaseText = template.Must(template.New("showcase").Parse(`{{if .Projects}}<h2 id="projects">Projects</h2>
<ul class="entries" role="list">
{{range .Projects}}<li>
{{.Picture}}
<h3><a href="{{.Repository}}">{{.Name}}</a></h3>
<p>{{.Line}}</p>
<p class="fine">Built with glonk {{.Glonk}}. License: {{.License}}.</p>
</li>
{{end}}</ul>
{{end}}<h2 id="show-your-project">Show your project</h2>
<p>A project is listed when:</p>
<ul>
{{range .Rule}}<li>{{.}}</li>
{{end}}</ul>
<p>{{.MaintainerMay}} To ask, <a href="{{.Form}}">fill in the form</a>: the link to the repository, a line on what it is, a screenshot or a recording, the version of glonk it builds with, and its license.</p>
<h2 id="the-badge">The badge</h2>
<p>For the README of something made with glonk: {{.Badge}}</p>
<p>Paste this line where it should stand:</p>
{{.Snippet}}<p>The badge is under the MIT license, as the logo is. <a href="` + repository + `/blob/main/BRAND.md">BRAND.md</a> says how the mark and the name may be used.</p>
`))

// The line a project pastes into its README to show the badge.
const badgeLine = "[![made with glonk](https://glonk.dev/badge.svg)](https://glonk.dev)"

// showcase is the page of projects made with glonk, from the list.
func showcase() (page, error) {
	data, err := os.ReadFile(root(showcaseList))
	if err != nil {
		return page{}, err
	}
	list, err := projects(data)
	if err != nil {
		return page{}, fmt.Errorf("%s: %w", showcaseList, err)
	}
	text, err := showcaseHTML(list)
	if err != nil {
		return page{}, fmt.Errorf("%s: %w", showcaseList, err)
	}
	return page{
		Path:        "site/showcase/index.html",
		Address:     "/showcase/",
		Title:       "Made with glonk",
		Description: "Projects built with glonk, a terminal rendering engine for Go, and how to have yours listed.",
		Front: `<h1>Made with glonk</h1>
<p class="lede">A place for what people build with glonk. If you have built something, show it here.</p>
`,
		Text: text,
	}, nil
}

// showcaseHTML checks the projects and gives the text of the page.
func showcaseHTML(list []project) (string, error) {
	released, err := releases()
	if err != nil {
		return "", err
	}
	type shown struct {
		project
		Picture template.HTML
	}
	var entries []shown
	names := map[string]bool{}
	for _, p := range list {
		if err := p.check(released); err != nil {
			return "", err
		}
		if names[p.Name] {
			return "", fmt.Errorf("%s is listed twice", p.Name)
		}
		names[p.Name] = true
		src := p.Picture
		if p.Recording != "" {
			src = p.Recording // picture shows its still, which opens it
		}
		img, err := picture(p.Alt, src, false)
		if err != nil {
			return "", fmt.Errorf("%s: %w", p.Name, err)
		}
		entries = append(entries, shown{p, template.HTML(img)})
	}

	// The badge is shown at its own size, which is in its file.
	badge, err := os.ReadFile(root("site/badge.svg"))
	if err != nil {
		return "", err
	}
	size := regexp.MustCompile(` width="(\d+)" height="(\d+)"`).FindSubmatch(badge)

	var out strings.Builder
	err = showcaseText.Execute(&out, map[string]any{
		"Projects":      entries,
		"Rule":          listingRule,
		"MaintainerMay": maintainerMay,
		"Form":          repository + "/issues/new?template=" + path.Base(showcaseForm),
		"Badge":         template.HTML(fmt.Sprintf(`<img class="badge" src="/badge.svg" width="%s" height="%s" alt="made with glonk">`, size[1], size[2])),
		"Snippet":       template.HTML(codeBlock([]string{badgeLine})),
	})
	return out.String(), err
}

// with gives a valid entry, changed by the function: the tests' starting point. Its pictures are
// two of the site's own.
func with(change func(*project)) project {
	p := project{
		Name:       "Cave",
		Repository: "https://example.com/someone/cave",
		Line:       "A cave to walk through, drawn in half blocks.",
		Picture:    "site/media/cube3d.png",
		Recording:  "site/media/cube3d.gif",
		Alt:        "A cave <of> blocks.",
		Glonk:      "v0.12.0",
		License:    "MIT",
	}
	if change != nil {
		change(&p)
	}
	return p
}

// inMedia points the showcase at site/media for a test, where there are pictures to list.
func inMedia(t *testing.T) {
	if _, err := os.Stat(root("site/media")); os.IsNotExist(err) {
		t.Skip("site/media is not in this copy of glonk")
	}
	folder := showcaseFolder
	showcaseFolder = "site/media/"
	t.Cleanup(func() { showcaseFolder = folder })
}

func TestAProjectIsShown(t *testing.T) {
	inMedia(t)
	got, err := showcaseHTML([]project{
		with(nil),
		with(func(p *project) { p.Name, p.Recording, p.Line = `Tom & "Jerry" <script>`, "", "A chase." }),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<h2 id="projects">Projects</h2>`,
		// A recording is shown as its still, which opens it.
		`<a href="/media/cube3d.gif"><img src="/media/cube3d.png" width="840" height="560" alt="A cave &lt;of&gt; blocks."></a>`,
		`<h3><a href="https://example.com/someone/cave">Cave</a></h3>`,
		`<p>A cave to walk through, drawn in half blocks.</p>`,
		`<p class="fine">Built with glonk v0.12.0. License: MIT.</p>`,
		// With no recording the picture stands alone, and what somebody else wrote is made safe.
		"<li>\n" + `<img src="/media/cube3d.png" width="840" height="560" alt="A cave &lt;of&gt; blocks.">`,
		`>Tom &amp; &#34;Jerry&#34; &lt;script&gt;</a></h3>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("wanted the page to hold %s, got:\n%s", want, got)
		}
	}
}

// Before the first project the page is an invitation: how to be listed and the badge, and no
// heading over a list that is not there.
func TestTheShowcaseWithNoProjectsIsAnInvitation(t *testing.T) {
	got, err := showcaseHTML(nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Projects") || strings.Contains(got, "entries") {
		t.Errorf("wanted no list of projects, got:\n%s", got)
	}
	for _, want := range append([]string{
		maintainerMay,
		`<a href="https://github.com/kyrokohan/glonk/issues/new?template=show_your_project.yml">fill in the form</a>`,
		`<img class="badge" src="/badge.svg" width="127" height="20" alt="made with glonk">`,
		`<pre tabindex="0" class="odd"><code>[![made with glonk](https://glonk.dev/badge.svg)](https://glonk.dev)` + "\n</code></pre>",
	}, listingRule...) {
		if !strings.Contains(got, want) {
			t.Errorf("wanted the page to hold %s, got:\n%s", want, got)
		}
	}
}

// An entry that is not as the rule and the page need it is refused, and the error names it.
func TestAProjectIsChecked(t *testing.T) {
	inMedia(t)
	maxPicture, maxRecording = 20<<10, 600<<10
	t.Cleanup(func() { maxPicture, maxRecording = 300<<10, 2<<20 })
	for name, change := range map[string]func(*project){
		"no name":                       func(p *project) { p.Name = " " },
		"no repository":                 func(p *project) { p.Repository = "" },
		"a repository not over https":   func(p *project) { p.Repository = "http://example.com/a/b" },
		"a repository that is a script": func(p *project) { p.Repository = "javascript:alert(1)" },
		"a repository with no host":     func(p *project) { p.Repository = "https:///a/b" },
		"no line":                       func(p *project) { p.Line = "" },
		"a line of two lines":           func(p *project) { p.Line = "One.\nTwo." },
		"a line too long":               func(p *project) { p.Line = strings.Repeat("é", 141) },
		"no words for the picture":      func(p *project) { p.Alt = "" },
		"no license":                    func(p *project) { p.License = "" },
		"a version that is no release":  func(p *project) { p.Glonk = "v9.9.9" },
		"a version with no v":           func(p *project) { p.Glonk = "0.12.0" },
		"no picture":                    func(p *project) { p.Picture, p.Recording = "", "" },
		"a picture outside the folder":  func(p *project) { p.Picture, p.Recording = "site/card.png", "" },
		"a picture reached by ..":       func(p *project) { p.Picture, p.Recording = "site/media/../card.png", "" },
		"a picture that is not there":   func(p *project) { p.Picture, p.Recording = "site/media/none.png", "" },
		"a picture that is too large":   func(p *project) { p.Picture, p.Recording = "site/media/world.png", ""; maxPicture = 10 << 10 },
		"a recording as the picture":    func(p *project) { p.Picture, p.Recording = "site/media/cube3d.gif", "" },
		"a recording of another name":   func(p *project) { p.Recording = "site/media/viewer.gif" },
		"a picture as the recording":    func(p *project) { p.Recording = "site/media/cube3d.png" },
		"a recording that is too large": func(p *project) { maxRecording = 500 << 10 },
	} {
		maxPicture, maxRecording = 20<<10, 600<<10
		p := with(change)
		_, err := showcaseHTML([]project{p})
		if err == nil {
			t.Errorf("%s: wanted the entry refused", name)
		} else if p.Name != " " && !strings.HasPrefix(err.Error(), p.Name+":") {
			t.Errorf("%s: wanted the error to start with the entry's name, got %v", name, err)
		}
	}
	maxPicture, maxRecording = 20<<10, 600<<10
	if _, err := showcaseHTML([]project{with(nil)}); err != nil {
		t.Errorf("wanted the entry as it is accepted, got %v", err)
	}
	// The line is counted in characters, not bytes.
	if _, err := showcaseHTML([]project{with(func(p *project) { p.Line = strings.Repeat("é", 140) })}); err != nil {
		t.Errorf("wanted a line of 140 characters accepted, got %v", err)
	}
	if _, err := showcaseHTML([]project{with(nil), with(func(p *project) { p.Line = "Another." })}); err == nil || !strings.Contains(err.Error(), "Cave is listed twice") {
		t.Errorf("wanted a name used twice refused, got %v", err)
	}
}

// A listed picture is a copy in the showcase's own folder, under a name that says what it is.
func TestAPictureIsInTheShowcaseFolder(t *testing.T) {
	for file, ext := range map[string]string{
		"site/media/cube3d.png":        ".png", // a picture of the site, but not of the showcase
		showcaseFolder + "cave.gif":    ".png",
		showcaseFolder + "cave.png":    ".gif",
		showcaseFolder + "../cave.png": ".png",
	} {
		if err := pictureFile(file, ext, "png", 1<<20); err == nil || !strings.Contains(err.Error(), "is not a "+ext+" file in "+showcaseFolder) {
			t.Errorf("%s: wanted it refused as not a %s file of the folder, got %v", file, ext, err)
		}
	}
}

// A file is what is in it, not what it is called.
func TestAPictureIsWhatIsInIt(t *testing.T) {
	if _, err := os.Stat(root("site/media")); os.IsNotExist(err) {
		t.Skip("site/media is not in this copy of glonk")
	}
	for _, c := range []struct {
		file, format string
		ok           bool
	}{
		{"site/media/cube3d.png", "png", true},
		{"site/media/cube3d.gif", "gif", true},
		{"site/media/cube3d.gif", "png", false},
		{"site/media/cube3d.png", "gif", false},
		{"site/media/fonts/Literata-OFL.txt", "png", false},
	} {
		if err := pictureIs(root(c.file), c.format, 1<<20); (err == nil) != c.ok {
			t.Errorf("%s as a %s picture: wanted ok %v, got %v", c.file, c.format, c.ok, err)
		}
	}
}

// The list is read strictly: a field that is misspelt is not quietly dropped.
func TestTheListIsReadStrictly(t *testing.T) {
	if list, err := projects([]byte(`[{"name": "Cave", "licence": "MIT"}]`)); err == nil {
		t.Errorf("wanted a field the list does not know refused, got %+v", list)
	}
	list, err := projects([]byte(`[{"name": "Cave", "license": "MIT"}]`))
	if err != nil || len(list) != 1 || list[0].Name != "Cave" || list[0].License != "MIT" {
		t.Errorf("wanted one project read, got %+v and %v", list, err)
	}
}

// The form asks for each line of the rule to be ticked, in the page's words, applies the label
// that marks a request, and says what the maintainer may do.
func TestTheFormHoldsTheRule(t *testing.T) {
	data, err := os.ReadFile(root(showcaseForm))
	if err != nil {
		t.Fatal(err)
	}
	form := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, line := range listingRule {
		if !strings.Contains(form, "        - label: "+line+"\n          required: true\n") {
			t.Errorf("%s: wanted a box that must be ticked for %q", showcaseForm, line)
		}
	}
	for _, want := range []string{maintainerMay, `labels: ["showcase"]`, "https://glonk.dev/showcase/"} {
		if !strings.Contains(form, want) {
			t.Errorf("%s: wanted %s", showcaseForm, want)
		}
	}
}

// The versions a project may name are the releases, not the section for what is not out yet.
func TestReleasesAreTheVersionsOfTheChangelog(t *testing.T) {
	got, err := releases()
	if err != nil {
		t.Fatal(err)
	}
	if !got["v0.12.0"] || !got["v0.1.0"] || got["vUnreleased"] || got["Unreleased"] || got["0.12.0"] {
		t.Errorf("wanted the releases as v0.12.0 and so on, got %v", got)
	}
}
