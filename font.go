package pdf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/speedata/fonts/type1"
)

var internalfontnumber int

type PDFFont struct {
	pw           *Writer
	InternalName string
	fontobject   *PDFObject
	FontFile     objectnumber
	filename     string
	usedChar     map[rune]bool
}

const (
	TYPE1 int = iota
	TRUETYPE
)

func guessFonttype(filename string) int {
	return TYPE1
}

func newInternalFontName() string {
	internalfontnumber++
	return fmt.Sprintf("/F%d", internalfontnumber)
}

func refOnum(onum objectnumber) string {
	return fmt.Sprintf("%d 0 R", onum)
}

type RuneSlice []rune

func (p RuneSlice) Len() int           { return len(p) }
func (p RuneSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p RuneSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Writes the font file to the PDF. The font should be sub-setted, therefore we know the requirements only the end of the PDF file.
func (fnt *PDFFont) finish() error {
	switch guessFonttype(fnt.filename) {
	case TYPE1:
		t1, err := type1.LoadFont(fnt.filename, "")
		if err != nil {
			return nil
		}

		usedChars := make(RuneSlice, len(fnt.usedChar))
		i := 0
		for g := range fnt.usedChar {
			usedChars[i] = g
			i++
		}
		sort.Sort(usedChars)
		charset, err := t1.Subset("AAAAAA", usedChars)
		if err != nil {
			return err
		}

		st := NewPDFStream(bytes.Join(t1.Segments, nil))
		st.dict = Dict{
			"/Length1": fmt.Sprintf("%d", len(t1.Segments[0])),
			"/Length2": fmt.Sprintf("%d", len(t1.Segments[1])),
			"/Length3": fmt.Sprintf("%d", len(t1.Segments[2])),
		}
		// pw = PDFWriter
		pw := fnt.pw
		fontfileObjectNumber := pw.writeStream(st)

		fontdescriptor := pw.NewPDFObject()
		fmt.Println(t1.FontBBox)
		fontdescriptor.Dict(Dict{
			"/Type":        "/FontDescriptor",
			"/FontName":    "/AAAAAA+" + t1.FontName,
			"/Flags":       "4",
			"/FontBBox":    fmt.Sprintf("[ %d %d %d %d ]", t1.FontBBox[0], t1.FontBBox[1], t1.FontBBox[2], t1.FontBBox[3]),
			"/ItalicAngle": fmt.Sprintf("%d", t1.ItalicAngle),
			"/Ascent":      fmt.Sprintf("%d", t1.Ascender),
			"/Descent":     fmt.Sprintf("%d", t1.Descender),
			"/CapHeight":   fmt.Sprintf("%d", t1.CapHeight),
			"/XHeight":     fmt.Sprintf("%d", t1.XHeight),
			"/StemV":       fmt.Sprintf("%d", 0),
			"/FontFile":    refOnum(fontfileObjectNumber),
			"/CharSet":     fmt.Sprintf("(%s)", charset),
		})
		fontdescriptor.Save()

		fontObj := fnt.fontobject

		widths := []string{"["}
		for i := usedChars[0]; i <= usedChars[len(usedChars)-1]; i++ {
			widths = append(widths, fmt.Sprintf("%d", t1.CharsCodepoint[i].Wx))
		}
		widths = append(widths, "]")
		wd := strings.Join(widths, " ")
		fdict := Dict{
			"/Type":           "/Font",
			"/Subtype":        "/Type1",
			"/BaseFont":       "/AAAAAA+" + t1.FontName,
			"/FirstChar":      fmt.Sprintf("%d", usedChars[0]),
			"/LastChar":       fmt.Sprintf("%d", usedChars[len(usedChars)-1]),
			"/Widths":         wd,
			"/FontDescriptor": refOnum(fontdescriptor.ObjectNumber),
		}
		fontObj.Dict(fdict)
		fontObj.Save()
	}
	return nil
}

// NewPDFFont registers a font for use in the PDF file.
func (pw *Writer) NewPDFFont(filename string) *PDFFont {
	f := &PDFFont{}
	f.usedChar = make(map[rune]bool)
	f.pw = pw
	f.InternalName = newInternalFontName()
	f.fontobject = pw.NewPDFObject()
	f.filename = filename
	pw.fonts = append(pw.fonts, f)
	return f
}
