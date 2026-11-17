// Package mesh draws 3D models through a perspective Camera onto an
// engine.FrameBuffer, as wireframes or as solid, lit surfaces.
//
// A [Mesh] holds vertices, the edges and triangular faces between them, where it
// sits in the world (its [Transform]) and how it looks (its [Material]). [Cube],
// [Sphere], [Cylinder], [Cone], [Plane], [Grid], [LoadOBJ] and [LoadOBJFS] make meshes. [Mesh.Draw]
// draws the edges, and [Mesh.DrawShaded] and [Mesh.DrawShadedPixels] draw the faces
// lit by any number of lights, each a [Light] such as [Directional], [Ambient] or
// [Point], or one of your own, into cells or into a half-block pixel Canvas.
//
// [Mesh.Draw] paints edges over whatever is already drawn. [Mesh.DrawDepth] gives each cell
// of each edge a depth instead, so nearer surfaces hide it and it hides farther ones, in
// whatever order things are drawn. [Mesh.DrawPixels] and [Mesh.DrawDepthPixels] draw the
// same edges into a half-block pixel Canvas, as lines twice as fine from top to bottom;
// call Canvas.Compose afterwards.
//
// A mesh with no [Mesh.Normals] is lit face by face, one brightness for each, so a
// round shape shows its facets. Give it a normal for each vertex and every cell is lit
// by itself, from the position and the normal blended across the face, so the shape
// looks round and a [Point] light near a big flat face makes a pool of light on it.
// [Sphere], [Cylinder], [Cone] and [Plane] come with normals, [LoadOBJ] reads them from
// a file, and [Mesh.SmoothNormals] works them out from the faces. Where a surface has
// a hard edge, as a cube does, give each side its own vertices so that each can face
// its own way.
//
// A mesh is one color, [Material.Color], unless it has [Mesh.FaceColors]: a color for
// each face, which replaces the material's for that face and is still lit. [LoadOBJFS]
// fills it from the materials of a model, reading the MTL files beside the OBJ from
// an [io/fs.FS]: the diffuse color of each, or the color of its texture picture (JPEG or
// PNG) at the middle of the face's texture coordinates, one color for each triangle.
// [LoadOBJ] skips materials.
//
// A face can have a picture instead: [Material.Texture] is a [bitmap.Bitmap], and
// [Mesh.FaceTexCoords] pins the three corners of a face to places in it, so that every cell
// or pixel of the face takes the color of the picture where it is, seen with perspective, and
// a wall of two triangles shows a picture of any size. The picture's color is lit as a face
// color is, and vertex colors tint it. A see-through pixel of the picture is a hole, where nothing
// is drawn and what is behind shows, as in a fence. [LoadOBJFS] fills both from a model's texture.
//
// A mesh can also be seen through as a whole: [Material.Opacity] is the part of its color that shows,
// and it is mixed over what is already drawn, as glass, water or a ghost is. Such a mesh is tested
// against depth and does not store it, so the solid meshes are drawn first and then the see-through
// ones, from far to near; nothing is sorted for you.
//
// [Mesh.VertexColors] gives each vertex a color, and a face fades from one corner's color
// to the next, blended at every cell with the same perspective-correct weights as smooth
// shading. A vertex color tints the color of a face that has one of its own, each channel
// multiplied, and is the color as it is where the face has none. [LoadOBJ] reads them from
// vertex lines of the form "v x y z r g b".
//
// Yaw and Pitch turn a model the way they turn a [Camera]: positive Yaw swings the
// front toward +x and positive Pitch tips it up.
//
// Coordinates: x points right, y up and z away from the viewer, so a Camera at the
// origin with Yaw and Pitch 0 looks down +z. Angles are in radians. OBJ files are
// right-handed and look down -z; LoadOBJ converts them.
//
// A light that reaches only so far can say so, with a method Reach, and is then not asked about
// a mesh that is wholly beyond it; [Point] does. See [Light].
//
// Drawing skips a mesh that cannot be on screen, by a sphere that holds its vertices. The
// shapes and models this package makes have it; a mesh built by hand with its vertices where
// they are in the world, as a level built in place is, calls [Mesh.UpdateBounds] once after
// setting them, and again after changing one in place.
//
// Drawing does not panic on a bad mesh, transform or camera. An edge or face that
// names a vertex the mesh does not have is skipped, and [Mesh.Validate] reports it, as
// it does a [Transform] that cannot be applied, which draws nothing. A [Camera] with
// a nonsense FOV, Near or CellAspect sees nothing, and one left at zero uses a
// default.
package mesh
