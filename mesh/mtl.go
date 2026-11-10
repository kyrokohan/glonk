package mesh

import (
	"io"
	"io/fs"
	"math"
	"path"
	"strconv"
	"strings"

	"glonk.dev/engine"
)

// The most an OBJ can ask of the MTL files it names. The files come from the same
// untrusted place as the model.
const (
	maxMTLBytes     = 1 << 20 // per file, or the OBJ's own limit if that is smaller
	maxMTLLibraries = 8       // mtllib names read per OBJ
	maxMTLMaterials = 10_000  // distinct material names kept per OBJ
)

// material is what an MTL file says about a material that this package uses.
type material struct {
	color   engine.Color // Kd, or the zero Color for none
	texture string       // map_Kd: an image file beside the OBJ, or "" for none
}

// materialLibrary holds the materials the MTL files named by an OBJ define, and the
// images they name, all found in fsys beside the OBJ.
type materialLibrary struct {
	fsys         fs.FS
	dir          string // the OBJ's directory: names in mtllib and map_Kd are relative to it
	limit        int64  // bytes read from each MTL file
	textureLimit int64  // bytes read from each image file
	libraries    int    // names tried so far, found or not

	materials map[string]material

	textures map[string]*texture // every image name tried so far; nil for one that failed
	pixels   int64               // decoded pixels spent on images so far; see maxTexturePixels
}

// newMaterialLibrary returns a library that reads files from fsys, beside the OBJ in dir.
// maxBytes is the limit given for the OBJ, which no file read for it may exceed.
func newMaterialLibrary(fsys fs.FS, dir string, maxBytes int64) *materialLibrary {
	return &materialLibrary{
		fsys:         fsys,
		dir:          dir,
		limit:        min(maxBytes, maxMTLBytes),
		textureLimit: min(maxBytes, maxTextureBytes),
		materials:    map[string]material{},
		textures:     map[string]*texture{},
	}
}

// load reads the MTL files that an mtllib statement names. A name that is not a plain
// path inside the OBJ's directory, a file that cannot be opened and one that is over
// the limit or cannot be read to the end are all skipped, the last two whole: half a
// file would color some faces of a model and not others.
func (l *materialLibrary) load(names []string) {
	for _, name := range names {
		if l.libraries >= maxMTLLibraries {
			return
		}
		l.libraries++

		if data, ok := l.readFile(name, l.limit); ok {
			l.parse(string(data))
		}
	}
}

// readFile reads the file called name beside the OBJ, up to limit bytes. It reports false
// for a name that is not a plain path inside the OBJ's directory, a file that cannot be
// opened, and one that is over the limit or cannot be read to the end: half a file is
// never returned.
func (l *materialLibrary) readFile(name string, limit int64) ([]byte, bool) {
	// Cleaning first accepts "./a.mtl" and leaves ".." only at the front, where
	// ValidPath refuses it. The backslash is refused so that a name means the same
	// on every system: it is a separator on Windows and an ordinary letter elsewhere.
	rel := path.Clean(name)
	if strings.ContainsRune(name, '\\') || !fs.ValidPath(rel) {
		return nil, false
	}
	f, err := l.fsys.Open(path.Join(l.dir, rel))
	if err != nil {
		return nil, false
	}
	data, err := io.ReadAll(&limitReader{r: f, left: limit})
	f.Close()
	return data, err == nil
}

// parse takes the materials of one MTL file: newmtl starts one, its Kd gives the color
// and its map_Kd the image. Everything else is skipped, and the later of two materials
// with one name wins.
func (l *materialLibrary) parse(data string) {
	var (
		name string
		open bool // a newmtl is in effect and the name has a place in materials
	)
	// The file is already capped, so there is no line limit to keep.
	for text := range strings.SplitSeq(data, "\n") {
		if i := strings.IndexByte(text, '#'); i >= 0 {
			text = text[:i]
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "newmtl":
			name = argument(text, "newmtl")
			_, known := l.materials[name]
			open = name != "" && (known || len(l.materials) < maxMTLMaterials)
			if open {
				l.materials[name] = material{} // a redefinition starts over
			}
		case "Kd":
			if open {
				m := l.materials[name]
				m.color = parseKd(fields[1:])
				l.materials[name] = m
			}
		case "map_Kd":
			if open {
				m := l.materials[name]
				m.texture = parseMapKd(argument(text, "map_Kd"))
				l.materials[name] = m
			}
		}
	}
}

// parseMapKd returns the image file name of a map_Kd statement, given what follows the
// keyword. The form with options (-s 1 1 1 file.png) is not read, so a name that starts
// with a dash gives none.
func parseMapKd(arg string) string {
	if strings.HasPrefix(arg, "-") {
		return ""
	}
	return arg
}

// argument returns what follows keyword on a line, the rest of it, as OBJ and MTL
// allow a name to hold spaces. Its caller has checked that the line starts with keyword.
func argument(line, keyword string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), keyword))
}

// parseKd reads the arguments of a Kd statement: three numbers for red, green and
// blue, or one for a gray, each from 0 to 1. Anything else, such as the spectral and
// xyz forms or a number that is not one, gives the zero Color, meaning none.
func parseKd(args []string) engine.Color {
	if len(args) != 1 && len(args) < 3 {
		return engine.Color{}
	}

	var c [3]uint8
	for i := range c {
		arg := args[min(i, len(args)-1)] // a single number is all three
		f, err := strconv.ParseFloat(arg, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return engine.Color{}
		}
		c[i] = uint8(math.Round(255 * min(max(f, 0), 1)))
	}
	return engine.RGB(c[0], c[1], c[2])
}
