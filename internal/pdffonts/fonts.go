package pdffonts

import _ "embed"

//go:embed DejaVuSans.ttf
var Regular []byte

//go:embed DejaVuSans-Bold.ttf
var Bold []byte

//go:embed DejaVuSans-Oblique.ttf
var Italic []byte
