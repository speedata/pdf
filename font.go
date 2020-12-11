package pdf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/speedata/fonts/type1"
)

var internalfontnumber int

// Font is any kind of font for the PDF file (currently only type1 is supported)
type Font struct {
	pw           *PDF
	InternalName string
	fontobject   *Object
	FontFile     objectnumber
	filename     string
	usedChar     map[rune]bool
}

const (
	// TYPE1 represents a afm/pfm based font
	TYPE1 int = iota
	// TRUETYPE is a TrueType based OpenType font
	TRUETYPE
)

func (fnt *Font) fonttype() int {
	return TYPE1
}

func newInternalFontName() string {
	internalfontnumber++
	return fmt.Sprintf("/F%d", internalfontnumber)
}

// Used for subsetting the fonts
type charSubset []rune

func (p charSubset) Len() int           { return len(p) }
func (p charSubset) Less(i, j int) bool { return p[i] < p[j] }
func (p charSubset) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// finish writes the font file to the PDF. The font should be sub-setted, therefore we know the requirements only the end of the PDF file.
func (fnt *Font) finish() error {
	switch fnt.fonttype() {
	case TYPE1:
		t1, err := type1.LoadFont(fnt.filename, "")
		if err != nil {
			return nil
		}

		subset := make(charSubset, len(fnt.usedChar))
		i := 0
		for g := range fnt.usedChar {
			subset[i] = g
			i++
		}
		sort.Sort(subset)
		charset, err := t1.Subset(subset)
		if err != nil {
			return err
		}

		st := NewStream(bytes.Join(t1.Segments, nil))
		st.dict = Dict{
			"/Length1": fmt.Sprintf("%d", len(t1.Segments[0])),
			"/Length2": fmt.Sprintf("%d", len(t1.Segments[1])),
			"/Length3": fmt.Sprintf("%d", len(t1.Segments[2])),
		}
		// pw = PDFWriter
		pw := fnt.pw
		fontfileObjectNumber := pw.writeStream(st)

		fontdescriptor := pw.NewObject()
		fontdescriptor.Dict(Dict{
			"/Type":        "/FontDescriptor",
			"/FontName":    "/" + t1.SubsetID + "+" + t1.FontName,
			"/Flags":       "4",
			"/FontBBox":    fmt.Sprintf("[ %d %d %d %d ]", t1.FontBBox[0], t1.FontBBox[1], t1.FontBBox[2], t1.FontBBox[3]),
			"/ItalicAngle": fmt.Sprintf("%d", t1.ItalicAngle),
			"/Ascent":      fmt.Sprintf("%d", t1.Ascender),
			"/Descent":     fmt.Sprintf("%d", t1.Descender),
			"/CapHeight":   fmt.Sprintf("%d", t1.CapHeight),
			"/XHeight":     fmt.Sprintf("%d", t1.XHeight),
			"/StemV":       fmt.Sprintf("%d", 0),
			"/FontFile":    fontfileObjectNumber.ref(),
			"/CharSet":     fmt.Sprintf("(%s)", charset),
		})
		fontdescriptor.Save()

		fontObj := fnt.fontobject

		widths := []string{"["}
		for i := subset[0]; i <= subset[len(subset)-1]; i++ {
			widths = append(widths, fmt.Sprintf("%d", t1.CharsCodepoint[i].Wx))
		}
		widths = append(widths, "]")
		wd := strings.Join(widths, " ")
		fdict := Dict{
			"/Type":           "/Font",
			"/Subtype":        "/Type1",
			"/BaseFont":       "/" + t1.SubsetID + "+" + t1.FontName,
			"/FirstChar":      fmt.Sprintf("%d", subset[0]),
			"/LastChar":       fmt.Sprintf("%d", subset[len(subset)-1]),
			"/Widths":         wd,
			"/FontDescriptor": fontdescriptor.ObjectNumber.ref(),
		}
		fontObj.Dict(fdict)
		fontObj.Save()
	}
	return nil
}

// NewFont registers a font for use in the PDF file.
func (pw *PDF) NewFont(filename string) *Font {
	f := &Font{}
	f.usedChar = make(map[rune]bool)
	f.pw = pw
	f.InternalName = newInternalFontName()
	f.fontobject = pw.NewObject()
	f.filename = filename
	pw.fonts = append(pw.fonts, f)
	return f
}
