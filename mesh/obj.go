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
// names and that sits beside the OBJ. Only the diffuse color, Kd, is read: three
// numbers from 0 to 1, or one for a gray. Textures and everything else about a
// material are skipped, so a textured material shows whatever plain Kd it has. The
// colors are used as they are, with no gamma conversion. A polygon cut into triangles
// gives them all its color.
//
// The MTL files are as untrusted as the OBJ, and a model that has no colors is still
// a model, so a problem with one does not fail the load; the faces it would have
// colored come out without. That covers a name that is not a plain path inside the
// OBJ's directory (a leading /, a leading .., a backslash), a file that is missing, a
// material that is not defined, a Kd that is not understood, and a file that is longer
// than 1 MiB (or maxBytes, if less) or cannot be read, which is skipped whole. At most
// 8 files are read per OBJ and 10,000 materials kept, so the memory of LoadOBJLimit
// grows by at most about 10 MiB while loading. An mtllib is read where it stands, so
// it must come before the usemtl that uses it. A file name with a space in it cannot
// be used, for spaces separate the names. os.DirFS follows symbolic links inside its
// folder, so give it a folder you trust.
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
		material   engine.Color       // the Kd of the usemtl in effect; zero for none
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
			v, err := parseOBJVertex(fields[1:])
			if err != nil {
				return fmt.Errorf("obj line %d: %w", line, err)
			}
			m.Vertices = append(m.Vertices, v)

		case "vn":
			n, err := parseOBJNumbers("normal", fields[1:])
			if err != nil {
				return fmt.Errorf("obj line %d: %w", line, err)
			}
			normals = append(normals, n)

		case "f":
			corners, anyNormal, err := parseOBJFace(cornerBuf[:0], fields[1:], len(m.Vertices), len(normals))
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
			// FaceColors starts at the first face with a color, and the faces before it
			// get none, so a file without any never has the list.
			if m.FaceColors == nil && !material.IsDefault() {
				m.FaceColors = make([]engine.Color, len(m.Faces))
			}
			// Fan from the first corner. Each triangle is listed backwards because
			// the Z flip mirrors the model.
			for i := 1; i+1 < len(corners); i++ {
				m.Faces = append(m.Faces, [3]int{corners[0].vertex, corners[i+1].vertex, corners[i].vertex})
			}
			for m.FaceColors != nil && len(m.FaceColors) < len(m.Faces) {
				m.FaceColors = append(m.FaceColors, material)
			}

		case "mtllib":
			if lib != nil {
				lib.load(fields[1:])
			}

		case "usemtl":
			// An unknown name, or none, means no color.
			if lib != nil {
				material = lib.materials[argument(text, "usemtl")].color
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

// parseOBJVertex reads the coordinates of a v statement. Anything after the first
// three numbers (an optional w, or vertex colors some exporters add) is ignored.
// Z is negated to convert from OBJ's right-handed space.
func parseOBJVertex(args []string) (geometry.Vector3, error) {
	return parseOBJNumbers("vertex", args)
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

// objCorner is one corner of a face: a vertex, and the normal the file gives it
// there, or -1 for none.
type objCorner struct {
	vertex, normal int
}

// parseOBJFace turns the corner tokens of an f statement ("7", "7/2", "7//3",
// "7/2/3") into corners with 0-based vertex indices, and normal indices where there
// are usable ones, appended to dst. haveVertices and haveNormals are how many
// vertices and normals have been read so far, which is what negative indices count
// back from. anyNormal says whether any corner has a normal.
//
// A vertex index that is not valid is an error. A normal index is not: one that is
// missing, is not a number or names no normal read so far leaves the corner without a
// normal, as an index into texture coordinates, which are never read, always has.
func parseOBJFace(dst []objCorner, args []string, haveVertices, haveNormals int) (corners []objCorner, anyNormal bool, err error) {
	for _, tok := range args {
		vertex, rest, _ := strings.Cut(tok, "/")
		i, err := parseOBJVertexIndex(vertex, tok, haveVertices)
		if err != nil {
			return dst, anyNormal, err
		}

		normal := -1
		if rest != "" {
			// What follows the vertex is "texture", "texture/normal" or "/normal".
			if _, field, found := strings.Cut(rest, "/"); found {
				normal = parseOBJNormalIndex(field, haveNormals)
				anyNormal = anyNormal || normal >= 0
			}
		}
		dst = append(dst, objCorner{vertex: i, normal: normal})
	}
	return dst, anyNormal, nil
}

// parseOBJNormalIndex reads the normal index field of a corner as a 0-based index, or
// -1 if it is not a usable one. have is how many normals have been read so far, which
// is what a negative index counts back from.
func parseOBJNormalIndex(field string, have int) int {
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
	}
	for i := range m.Faces {
		for k, v := range m.Faces[i] {
			if v < 0 {
				m.Faces[i][k] = places - v - 1
			}
		}
	}
}
