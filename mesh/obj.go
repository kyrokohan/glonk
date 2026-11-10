package mesh

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path"
	"strconv"
	"strings"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// maxOBJLine is the longest logical line LoadOBJ accepts, in bytes. A polygon with
// tens of thousands of corners still fits; anything longer is more likely a
// corrupt or hostile file than a model.
const maxOBJLine = 1 << 20

// LoadOBJ reads a Wavefront OBJ model, the plain-text format nearly every 3D tool
// can export, and returns it as a Mesh ready to draw.
//
// It understands vertices (v), normals (vn), faces (f, in the v, v/vt, v//vn and
// v/vt/vn forms) and line elements (l). Texture coordinates, materials, groups and
// anything else are skipped, and all objects in the file are merged into one Mesh.
//
// A vertex line of the form "v x y z r g b", exactly six numbers, gives the vertex a
// color, which goes in Mesh.VertexColors: red, green and blue from 0 to 1, clamped, as a
// material's Kd is read. Any other count gives no color (a w after the coordinates is
// one), and so do three numbers that are not understood; neither fails the load. Vertex
// colors are in the vertex line, so LoadOBJLimit and LoadOBJFS read them too. A file whose
// vertices have none has no VertexColors, and one where only some do has the zero Color
// for the others. A vertex that is copied for a normal (see below) keeps its color.
//
// Indices start at 1, and a negative index counts back from the vertices (or
// normals) read so far. A face with more than three corners is cut into triangles,
// which are used for shading, while Edges follows the face's outline so a wireframe
// shows the original polygons, with shared edges listed once.
//
// Materials (mtllib, usemtl) are skipped as well, because an MTL file sits beside the
// OBJ and a reader has nowhere to look for it. LoadOBJFS reads them.
//
// Normals make the Mesh shaded smooth (see Mesh.Normals). A Mesh holds one normal per
// vertex, while an OBJ file gives each corner of each face its own, so where a file
// lights a vertex differently on different faces (the sides of a cube, say) it comes
// back as one vertex for each normal it has, at the same place. Edges still joins the
// original corners, so a wireframe looks the same. A corner that has no normal, or
// whose normal index is not a number or names a normal not read yet, gets a zero
// normal, and shading gives it the face's own. A file whose faces use no normals
// loads as it always did, with no Normals.
//
// OBJ files are right-handed and look down -Z, while this package looks down +Z.
// LoadOBJ converts: it negates every Z, normals' too, and reverses each face's
// corner order, so the outside of a model stays its outside. A model that looks
// inside out was wound the other way in the file; set Mesh.DoubleSided to draw it
// anyway.
//
// The input is not trusted. Malformed numbers, bad vertex indices and the like
// return an error naming the line, and a file with no faces and no lines is an
// error. The time taken grows in step with the size of the input. Memory use is
// roughly 25 times the input size (a 3.7 MB file of quads used 93 MB), and up to
// roughly 35 times for a file that gives most corners a normal of their own (a 7.3 MB
// file of quads like that used 233 MB). LoadOBJ reads until the reader ends, so for a
// source you do not control use LoadOBJLimit.
func LoadOBJ(r io.Reader) (Mesh, error) {
	return readOBJ(r, 0, nil)
}

// ErrOBJTooLarge is wrapped by the error LoadOBJLimit returns when its input is
// longer than the limit.
var ErrOBJTooLarge = errors.New("obj: input too large")

// LoadOBJLimit is LoadOBJ for input you do not control. It reads at most maxBytes
// bytes; if the reader has more it returns an error wrapping ErrOBJTooLarge rather
// than a half-loaded model, which wrapping the reader in io.LimitReader would
// produce. maxBytes must be positive. As a guide, the memory used is roughly 35
// times maxBytes in the worst case. Like LoadOBJ it skips materials; LoadOBJFS reads
// them.
//
// An over-long input always fails. Usually the error wraps ErrOBJTooLarge, but a
// malformed statement found before the limit is reached is reported as itself.
func LoadOBJLimit(r io.Reader, maxBytes int64) (Mesh, error) {
	if maxBytes <= 0 {
		return Mesh{}, fmt.Errorf("obj: limit must be positive, got %d", maxBytes)
	}
	return readOBJ(r, maxBytes, nil)
}

// LoadOBJFS is LoadOBJLimit for a model that comes with materials: it opens the OBJ
// called name in fsys (an os.DirFS, an embed.FS, a zip) and gives each face the color
// of its material, in Mesh.FaceColors. maxBytes limits the OBJ as it does for
// LoadOBJLimit, and name must satisfy fs.ValidPath.
//
// A material is named by usemtl, and defined in an MTL file that an mtllib statement
// names and that sits beside the OBJ. Two things in it are read. Kd is the diffuse color:
// three numbers from 0 to 1, or one for a gray. map_Kd is a picture, a JPEG or PNG file
// beside the OBJ (found by what is in it, not by its name), and the plain form is the
// only one read: a name that starts with a dash, as when options come first, is skipped.
// Everything else about a material is skipped. Colors are used as they are, with no
// gamma conversion. A polygon cut into triangles gives them all its color.
//
// A picture colors a triangle through the texture coordinates (vt) of its corners: the
// color is that of the nearest pixel of the picture at the middle of the three. The
// coordinates are as an OBJ gives them, (0, 0) at the bottom left and (1, 1) at the top
// right, and the picture repeats beyond that. The picture's own transparency is ignored.
// A triangle takes the picture's color where a corner is missing its coordinates (or
// they are not numbers, or name none that was read), the picture could not be had or the
// material has none, and then the Kd of the material, if it has one, so a material with
// both shows the picture where it can. One color for each triangle means a large
// triangle is one flat color, however fine the picture. The coordinates are read as the
// file comes, so an mtllib and the vt lines must come before the faces that use them, and
// a mtllib before the usemtl that uses it.
//
// Vertex colors (see LoadOBJ) are read as well and kept apart from these: they are in
// Mesh.VertexColors and tint the color a face gets here, as that field says.
//
// The MTL files and pictures are as untrusted as the OBJ, and a model that has no colors
// is still a model, so a problem with one does not fail the load; the faces it would
// have colored come out without. That covers a name that is not a plain path inside the
// OBJ's directory (a leading /, a leading .., a backslash), a file that is missing, a
// material that is not defined, a Kd that is not understood, a file that is longer than
// 1 MiB (or maxBytes, if less) for an MTL and 64 MiB (or maxBytes) for a picture or
// cannot be read, which is skipped whole, a picture that is not a JPEG or PNG or is
// broken or cut short, and a picture too large: the size is read from its header and
// refused before anything is decoded, and 16,777,216 pixels (4096 by 4096) may be decoded
// for one OBJ in all, including those of a picture that then fails. At most 8 MTL files
// and 8 pictures are read per OBJ, and 10,000 materials kept. The memory of LoadOBJLimit
// grows by at most about 10 MiB for the MTL files, and by a decoded picture while the
// model loads, up to 128 MiB for the largest allowed, and the 16 bytes of each vt line,
// which are freed with the load. Pictures are read when the first face that uses one
// comes, once. An mtllib is read where it stands. A file name with a space in it cannot
// be used for an mtllib, for spaces separate the names. os.DirFS follows symbolic links
// inside its folder, so give it a folder you trust.
//
// A model with no usable material loads as LoadOBJLimit would, with no FaceColors.
func LoadOBJFS(fsys fs.FS, name string, maxBytes int64) (Mesh, error) {
	if maxBytes <= 0 {
		return Mesh{}, fmt.Errorf("obj: limit must be positive, got %d", maxBytes)
	}
	if !fs.ValidPath(name) {
		return Mesh{}, fmt.Errorf("obj: invalid file name %q", name)
	}
	f, err := fsys.Open(name)
	if err != nil {
		return Mesh{}, err
	}
	defer f.Close()

	return readOBJ(f, maxBytes, newMaterialLibrary(fsys, path.Dir(name), maxBytes))
}

// limitReader passes through at most limit bytes and fails on the first byte past
// that, instead of ending the stream quietly the way io.LimitReader does.
type limitReader struct {
	r    io.Reader
	left int64 // bytes still allowed
	over bool  // the source had more than the limit
}

func (l *limitReader) Read(p []byte) (int, error) {
	if l.over {
		return 0, ErrOBJTooLarge
	}

	// Ask for one byte more than allowed: receiving it is how we know the input is
	// too long, as opposed to exactly as long as the limit.
	if l.left < math.MaxInt64 && int64(len(p)) > l.left+1 {
		p = p[:l.left+1]
	}
	n, err := l.r.Read(p)
	if int64(n) > l.left {
		l.over = true
		return 0, ErrOBJTooLarge
	}
	l.left -= int64(n)
	return n, err
}

// readOBJ is LoadOBJ, LoadOBJLimit and LoadOBJFS; a limit of 0 means none. Materials
// are read from lib, which is nil for the first two: they skip mtllib and usemtl.
func readOBJ(r io.Reader, limit int64, lib *materialLibrary) (Mesh, error) {
	var (
		m          Mesh
		seen       = map[[2]int]bool{}
		lineNo     int             // the line being read
		startLine  int             // the line a continued statement began on
		continued  strings.Builder // the text of a statement carried over from lines ending in \
		hasPending bool
		normals    []geometry.Vector3 // every vn, Z already negated
		sm         smoothing          // how corners with normals become vertices; see there
		cornerBuf  []objCorner        // the corners of the face being read, kept to read the next
		uvs        [][2]float64       // every vt, when materials are being read; see parseOBJUV
		mat        material           // the usemtl in effect; zero for none
		tex        *texture           // mat's image, once it has been asked for
		texAsked   bool               // tex has been asked for since mat last changed
	)

	// The scanner reads through lr. When the limit is hit it sees a read error, which
	// it treats as the end of the input, so it first hands over the cut-off last
	// line. That line can fail to parse, and the parse error would hide the real
	// reason. fail puts the right error back wherever one is returned.
	var lr *limitReader
	if limit > 0 {
		lr = &limitReader{r: r, left: limit}
		r = lr
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxOBJLine)
	// Both checks after the loop run only when the scanner reported no error, so
	// the limit cannot have been hit there and they return their errors as they are.
	fail := func(err error) (Mesh, error) {
		if lr != nil && lr.over {
			return Mesh{}, fmt.Errorf("%w: more than %d bytes", ErrOBJTooLarge, limit)
		}
		return Mesh{}, err
	}

	addEdge := func(a, b int) {
		if a == b {
			return
		}
		if a > b {
			a, b = b, a
		}
		if key := [2]int{a, b}; !seen[key] {
			seen[key] = true
			m.Edges = append(m.Edges, key)
		}
	}

	statement := func(line int, text string) error {
		if i := strings.IndexByte(text, '#'); i >= 0 {
			text = text[:i]
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			return nil
		}

		switch fields[0] {
		case "v":
			v, color, err := parseOBJVertex(fields[1:])
			if err != nil {
				return fmt.Errorf("obj line %d: %w", line, err)
			}
			m.Vertices = append(m.Vertices, v)
			// VertexColors starts at the first vertex with a color, and the vertices before
			// it get none, so a file without any never has the list.
			if m.VertexColors == nil && !color.IsDefault() {
				m.VertexColors = make([]engine.Color, len(m.Vertices)-1)
			}
			if m.VertexColors != nil {
				m.VertexColors = append(m.VertexColors, color)
			}

		case "vn":
			n, err := parseOBJNumbers("normal", fields[1:])
			if err != nil {
				return fmt.Errorf("obj line %d: %w", line, err)
			}
			normals = append(normals, n)

		case "vt":
			// Only a model with materials can use them, and the others stay as lean as
			// they were.
			if lib != nil {
				uvs = append(uvs, parseOBJUV(fields[1:]))
			}

		case "f":
			corners, anyNormal, err := parseOBJFace(cornerBuf[:0], fields[1:], len(m.Vertices), len(normals), len(uvs))
			cornerBuf = corners
			if err != nil {
				return fmt.Errorf("obj line %d: %w", line, err)
			}
			if len(corners) < 3 {
				return fmt.Errorf("obj line %d: a face needs at least 3 corners, got %d", line, len(corners))
			}
			// Edges join the corners' places, whatever their normals.
			for i, c := range corners {
				addEdge(c.vertex, corners[(i+1)%len(corners)].vertex)
			}
			// Each corner is drawn with the place itself, or with a copy of it made for
			// the normal the corner has there. A face without normals, in a file without
			// them so far, needs nothing done.
			if anyNormal || sm.active {
				for i, c := range corners {
					corners[i].vertex = sm.vertexFor(c, &m)
				}
			}
			// The image is read when the first face that uses it comes, so one that no face
			// uses is never opened.
			if !texAsked && mat.texture != "" {
				tex, texAsked = lib.texture(mat.texture), true
			}
			// Fan from the first corner. Each triangle is listed backwards because
			// the Z flip mirrors the model.
			for i := 1; i+1 < len(corners); i++ {
				m.Faces = append(m.Faces, [3]int{corners[0].vertex, corners[i+1].vertex, corners[i].vertex})

				c := mat.color
				if tex != nil {
					if t := tex.at(uvCentroid(uvs, corners[0], corners[i], corners[i+1])); !t.IsDefault() {
						c = t
					}
				}
				// FaceColors starts at the first face with a color, and the faces before it
				// get none, so a file without any never has the list.
				if m.FaceColors == nil && !c.IsDefault() {
					m.FaceColors = make([]engine.Color, len(m.Faces)-1)
				}
				if m.FaceColors != nil {
					m.FaceColors = append(m.FaceColors, c)
				}
			}

		case "mtllib":
			if lib != nil {
				lib.load(fields[1:])
			}

		case "usemtl":
			// An unknown name, or none, means no color.
			if lib != nil {
				mat, tex, texAsked = lib.materials[argument(text, "usemtl")], nil, false
			}

		case "l":
			corners, err := parseOBJCorners(fields[1:], len(m.Vertices))
			if err != nil {
				return fmt.Errorf("obj line %d: %w", line, err)
			}
			if len(corners) < 2 {
				return fmt.Errorf("obj line %d: a line needs at least 2 points, got %d", line, len(corners))
			}
			for i := 0; i+1 < len(corners); i++ {
				addEdge(corners[i], corners[i+1])
			}
		}
		return nil
	}

	for sc.Scan() {
		lineNo++
		text := sc.Text()

		if !hasPending {
			startLine = lineNo
		}
		// Pieces go into a Builder so each is copied once. Gluing them with string
		// concatenation copied everything so far for every piece, which made a long
		// run of continued lines take time proportional to its square.
		if strings.HasSuffix(text, "\\") {
			continued.WriteString(strings.TrimSuffix(text, "\\"))
			continued.WriteByte(' ')
			hasPending = true
			if continued.Len() > maxOBJLine {
				return fail(fmt.Errorf("obj line %d: %w", startLine, bufio.ErrTooLong))
			}
			continue
		}

		continued.WriteString(text)
		err := statement(startLine, continued.String())
		continued.Reset()
		hasPending = false
		if err != nil {
			return fail(err)
		}
	}
	if err := sc.Err(); err != nil {
		return fail(fmt.Errorf("obj line %d: %w", lineNo+1, err))
	}
	if hasPending { // the file ended on a backslash
		if err := statement(startLine, continued.String()); err != nil {
			return Mesh{}, err
		}
	}

	if len(m.Faces) == 0 && len(m.Edges) == 0 {
		return Mesh{}, errors.New("obj: no faces or lines")
	}
	sm.finish(&m, normals)
	return m, nil
}

// parseOBJVertex reads a v statement: the coordinates, with Z negated to convert from
// OBJ's right-handed space, and the vertex color some exporters add after them. A color
// is exactly three more numbers, from 0 to 1 as Kd is read; with any other count, such as
// a w after the coordinates, or numbers that are not understood, the vertex has none and
// the zero Color is returned, and only bad coordinates are an error.
func parseOBJVertex(args []string) (geometry.Vector3, engine.Color, error) {
	v, err := parseOBJNumbers("vertex", args)
	if err != nil || len(args) != 6 {
		return v, engine.Color{}, err
	}
	return v, parseKd(args[3:]), nil
}

// parseOBJNumbers reads the three numbers of a v or vn statement, naming what it is
// reading in an error, and negates Z to convert from OBJ's right-handed space.
func parseOBJNumbers(what string, args []string) (geometry.Vector3, error) {
	if len(args) < 3 {
		return geometry.Vector3{}, fmt.Errorf("a %s needs 3 numbers, got %d", what, len(args))
	}

	var c [3]float64
	for i := range c {
		f, err := strconv.ParseFloat(args[i], 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return geometry.Vector3{}, fmt.Errorf("bad number %q", args[i])
		}
		c[i] = f
	}
	return geometry.Vector3{X: c[0], Y: c[1], Z: -c[2]}, nil
}

// parseOBJCorners turns the corner tokens of an l statement ("7", "7/2") into 0-based
// vertex indices. have is how many vertices have been read so far, which is what
// negative indices count back from.
func parseOBJCorners(args []string, have int) ([]int, error) {
	corners := make([]int, 0, len(args))

	for _, tok := range args {
		vertex, _, _ := strings.Cut(tok, "/")
		i, err := parseOBJVertexIndex(vertex, tok, have)
		if err != nil {
			return nil, err
		}
		corners = append(corners, i)
	}
	return corners, nil
}

// parseOBJVertexIndex reads vertex, the part of the corner token tok before its first
// slash, as a 0-based index. have is how many vertices have been read so far, which is
// what a negative index counts back from.
func parseOBJVertexIndex(vertex, tok string, have int) (int, error) {
	if vertex == "" {
		return 0, fmt.Errorf("missing vertex index in %q", tok)
	}
	n, err := strconv.Atoi(vertex)
	if err != nil {
		return 0, fmt.Errorf("bad vertex index %q", vertex)
	}

	i := n - 1
	if n < 0 {
		i = have + n
	}
	if i < 0 || i >= have { // index 0 lands on i = -1
		return 0, fmt.Errorf("vertex index %d out of range (%d vertices so far)", n, have)
	}
	return i, nil
}

// objCorner is one corner of a face: a vertex, and the normal and texture coordinates
// the file gives it there, or -1 for none.
type objCorner struct {
	vertex, normal, texture int
}

// parseOBJFace turns the corner tokens of an f statement ("7", "7/2", "7//3",
// "7/2/3") into corners with 0-based vertex indices, and normal and texture indices
// where there are usable ones, appended to dst. haveVertices, haveNormals and haveUVs
// are how many vertices, normals and texture coordinates have been read so far, which is
// what negative indices count back from. anyNormal says whether any corner has a normal.
//
// A vertex index that is not valid is an error. A normal or texture index is not: one
// that is missing, is not a number or names none read so far leaves the corner without
// one. A file read without materials counts no texture coordinates, so none is usable.
func parseOBJFace(dst []objCorner, args []string, haveVertices, haveNormals, haveUVs int) (corners []objCorner, anyNormal bool, err error) {
	for _, tok := range args {
		vertex, rest, _ := strings.Cut(tok, "/")
		i, err := parseOBJVertexIndex(vertex, tok, haveVertices)
		if err != nil {
			return dst, anyNormal, err
		}

		normal, texture := -1, -1
		if rest != "" {
			// What follows the vertex is "texture", "texture/normal" or "/normal".
			field, normalField, found := strings.Cut(rest, "/")
			texture = parseOBJOptionalIndex(field, haveUVs)
			if found {
				normal = parseOBJOptionalIndex(normalField, haveNormals)
				anyNormal = anyNormal || normal >= 0
			}
		}
		dst = append(dst, objCorner{vertex: i, normal: normal, texture: texture})
	}
	return dst, anyNormal, nil
}

// parseOBJOptionalIndex reads the normal or texture index field of a corner as a
// 0-based index, or -1 if it is not a usable one. have is how many normals or texture
// coordinates have been read so far, which is what a negative index counts back from.
func parseOBJOptionalIndex(field string, have int) int {
	// Nothing is usable with none read, and an empty field is the common "v//vn", whose
	// failed Atoi would allocate an error for every corner.
	if have == 0 || field == "" {
		return -1
	}
	n, err := strconv.Atoi(field)
	if err != nil {
		return -1
	}
	i := n - 1
	if n < 0 {
		i = have + n
	}
	if i < 0 || i >= have { // index 0 lands on i = -1
		return -1
	}
	return i
}

// parseOBJUV reads the arguments of a vt statement: u, then v (0 if it is left out), and
// a w that is ignored. A statement that is not understood gives NaN, which no texture
// has a color for, instead of failing the load: it still takes its place in the list, so
// the indices after it mean what the file meant.
func parseOBJUV(args []string) [2]float64 {
	uv := [2]float64{math.NaN(), math.NaN()}
	if len(args) == 0 {
		return uv
	}
	var c [2]float64
	for i := range min(len(args), 2) {
		f, err := strconv.ParseFloat(args[i], 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return uv
		}
		c[i] = f
	}
	return c
}

// uvCentroid is the middle of the texture coordinates of three corners, or NaN if one
// of them has none.
func uvCentroid(uvs [][2]float64, a, b, c objCorner) (u, v float64) {
	if a.texture < 0 || b.texture < 0 || c.texture < 0 {
		return math.NaN(), math.NaN()
	}
	p, q, r := uvs[a.texture], uvs[b.texture], uvs[c.texture]
	return (p[0] + q[0] + r[0]) / 3, (p[1] + q[1] + r[1]) / 3
}

// smoothing turns the corners of faces that have normals into vertices. A Mesh has
// one normal per vertex, so a place that a file gives several normals needs a copy for
// each: the first normal a place is used with takes the place's own vertex, and every
// other takes a copy at the end. A file whose faces use no normals never wakes this up,
// and keeps every vertex where the file put it.
type smoothing struct {
	active bool // some corner has had a normal

	// own[v] is the normal that vertex v was given (none for a corner without one),
	// or unclaimed if no corner has used it since this woke up.
	own []int

	// copies are the vertices made for places used with a second normal, in order:
	// the place and the normal. The faces that use them hold a placeholder, -1 for the
	// first copy, -2 for the second and so on, until finish knows where they go.
	copies [][2]int
	copyOf map[[2]int]int // (place, normal) to its placeholder
}

const (
	noNormal  = -1
	unclaimed = -2
)

// vertexFor returns the vertex to draw corner c with, now that normals are in use.
func (s *smoothing) vertexFor(c objCorner, m *Mesh) int {
	if !s.active {
		s.wake(m)
	}
	for len(s.own) < len(m.Vertices) {
		s.own = append(s.own, unclaimed)
	}

	switch s.own[c.vertex] {
	case unclaimed:
		s.own[c.vertex] = c.normal
		return c.vertex
	case c.normal:
		return c.vertex
	}

	key := [2]int{c.vertex, c.normal}
	if id, ok := s.copyOf[key]; ok {
		return id
	}
	s.copies = append(s.copies, key)
	id := -len(s.copies)
	s.copyOf[key] = id
	return id
}

// wake starts the bookkeeping at the first corner with a normal. Every face so far was
// made without normals, so each vertex they use belongs to the corners without one.
func (s *smoothing) wake(m *Mesh) {
	s.active = true
	s.copyOf = map[[2]int]int{}
	s.own = make([]int, len(m.Vertices))
	for i := range s.own {
		s.own[i] = unclaimed
	}
	for _, f := range m.Faces {
		for _, v := range f {
			s.own[v] = noNormal
		}
	}
}

// finish gives the mesh its Normals and its copies of vertices, and puts the copies'
// real indices in the faces that hold placeholders. It does nothing if no corner had a
// normal.
func (s *smoothing) finish(m *Mesh, normals []geometry.Vector3) {
	if !s.active {
		return
	}
	places := len(m.Vertices)
	m.Normals = make([]geometry.Vector3, places, places+len(s.copies))
	for v, n := range s.own {
		if n >= 0 {
			m.Normals[v] = normals[n]
		}
	}
	for _, c := range s.copies {
		var n geometry.Vector3
		if c[1] >= 0 {
			n = normals[c[1]]
		}
		m.Vertices = append(m.Vertices, m.Vertices[c[0]])
		m.Normals = append(m.Normals, n)
		if m.VertexColors != nil {
			m.VertexColors = append(m.VertexColors, m.VertexColors[c[0]])
		}
	}
	for i := range m.Faces {
		for k, v := range m.Faces[i] {
			if v < 0 {
				m.Faces[i][k] = places - v - 1
			}
		}
	}
}
