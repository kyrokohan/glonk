// The recordings of the examples that the README and glonk.dev show. This folder is a module of
// its own for one reason: Go leaves a folder with a go.mod out of the module around it, so a
// go get of glonk does not download pictures. Nothing imports it.
module glonk.dev/site/media

go 1.26.0
