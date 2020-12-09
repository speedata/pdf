package pdf

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type objectnumber int

type Dict map[string]string

type Writer struct {
	outfile         io.WriteSeeker
	nextobject      objectnumber
	objectlocations map[objectnumber]int64
	pages           *PDFPages
	lastEOL         int64
	fonts           []*PDFFont
}

func NewWriter(file io.WriteSeeker) *Writer {
	pw := Writer{}
	pw.outfile = file
	pw.nextobject = 1
	pw.objectlocations = make(map[objectnumber]int64)
	pw.pages = &PDFPages{}
	pw.out("%PDF-1.7")
	return &pw
}

type PDFPages struct {
	pages []*PDFPage
	dict  objectnumber
}

type PDFPage struct {
	onum   objectnumber
	dict   objectnumber
	Fonts  []*PDFFont
	stream *PDFStream
}

// Register the used characters
func (pg *PDFPage) Runes(fnt *PDFFont, r string) {
	for _, v := range r {
		fnt.usedChar[v] = true
	}
	fmt.Println(fnt.usedChar)
}

// Add page to the file. The stream must be complete
func (pw *Writer) AddPage(pagestream *PDFStream) *PDFPage {
	pg := &PDFPage{}
	pg.stream = pagestream
	pw.pages.pages = append(pw.pages.pages, pg)
	return pg
}

// Get next free object number
func (pw *Writer) nextObject() objectnumber {
	pw.nextobject++
	return pw.nextobject - 1
}

func (pw *Writer) writeStream(st *PDFStream) objectnumber {
	obj := pw.NewPDFObject()
	st.dict["/Length"] = fmt.Sprintf("%d", len(st.data))
	obj.Dict(st.dict)
	obj.Data.WriteString("\nstream\n")
	obj.Data.Write(st.data)
	obj.Data.WriteString("\nendstream\n")
	obj.Save()
	return obj.ObjectNumber
}

func (pw *Writer) writeDocumentCatalog() (objectnumber, error) {
	// Write all page streams:
	for _, page := range pw.pages.pages {
		page.onum = pw.writeStream(page.stream)
	}

	// Page streams are finished. Now the /Page dictionaries with
	// references to the streams and the parent
	// Pages objects have to be placed in the file

	//  We need to know in advance where the parent object is written (/Pages)
	pagesObj := pw.NewPDFObject()

	for _, page := range pw.pages.pages {
		obj := pw.NewPDFObject()
		onum := obj.ObjectNumber
		page.dict = onum

		var res []string
		if len(page.Fonts) > 0 {
			res = append(res, "<< ")
			for _, fnt := range page.Fonts {
				res = append(res, fmt.Sprintf("%s %d 0 R", fnt.InternalName, fnt.fontobject.ObjectNumber))
			}
			res = append(res, " >>")
		}

		resHash := Dict{}
		if len(page.Fonts) > 0 {
			resHash["/Font"] = strings.Join(res, " ")
		}
		pageHash := Dict{
			"/Type":     "/Page",
			"/Contents": fmt.Sprintf("%d 0 R", page.onum),
			"/Parent":   fmt.Sprintf("%d 0 R", pagesObj.ObjectNumber),
		}

		resHash["/ProcSet"] = "[ /PDF /Text ]"
		if len(resHash) > 0 {
			pageHash["/Resources"] = hashToString(resHash, 1)
		}
		obj.Dict(pageHash)
		obj.Save()
	}

	// The pages object
	kids := make([]string, len(pw.pages.pages))
	for i, v := range pw.pages.pages {
		kids[i] = fmt.Sprintf("%d 0 R", v.dict)
	}

	fmt.Fprintln(pw.outfile, "%% The pages object")
	pw.pages.dict = pagesObj.ObjectNumber
	pagesObj.Dict(Dict{
		"/Type":  "/Pages",
		"/Kids":  "[ " + strings.Join(kids, " ") + " ]",
		"/Count": fmt.Sprint(len(pw.pages.pages)),
		// "/Resources": "<<  >>",
		"/MediaBox": "[0 0 612 792]",
	})
	pagesObj.Save()

	catalog := pw.NewPDFObject()
	catalog.Dict(Dict{
		"/Type":  "/Catalog",
		"/Pages": fmt.Sprintf("%d 0 R", pw.pages.dict),
	})
	catalog.Save()

	for _, fnt := range pw.fonts {
		fnt.finish()
	}
	return catalog.ObjectNumber, nil
}

// Does not close the file
func (pw *Writer) Finish() error {
	fmt.Println("Now finishing the PDF")
	dc, err := pw.writeDocumentCatalog()
	if err != nil {
		return err
	}

	// XRef section
	xrefpos := pw.curpos()
	pw.out("xref")
	pw.outf("0 %d\n", pw.nextobject)
	fmt.Fprintln(pw.outfile, "0000000000 65535 f ")
	for i := objectnumber(1); i < pw.nextobject; i++ {
		if loc, ok := pw.objectlocations[i]; ok {
			fmt.Fprintf(pw.outfile, "%010d 00000 n \n", loc)
		}
	}

	trailer := Dict{
		"/Size": fmt.Sprint(pw.nextobject - 1),
		"/Root": fmt.Sprintf("%d 0 R", dc),
		"/ID":   "[<72081BF410BDCCB959F83B2B25A355D7> <72081BF410BDCCB959F83B2B25A355D7>]",
	}
	fmt.Fprintln(pw.outfile, "trailer")
	pw.outHash(trailer)
	fmt.Fprintln(pw.outfile, "startxref")
	fmt.Fprintf(pw.outfile, "%d\n", xrefpos)
	fmt.Fprintln(pw.outfile, "%%EOF")
	return nil
}

func hashToString(h Dict, level int) string {
	var b bytes.Buffer
	b.WriteString(strings.Repeat("  ", level))
	b.WriteString("<<\n")
	for k, v := range h {
		b.WriteString(fmt.Sprintf("%s%s %s\n", strings.Repeat("  ", level+1), k, v))
	}
	b.WriteString(strings.Repeat("  ", level))
	b.WriteString(">>")
	return b.String()
}

func (pw *Writer) outHash(h Dict) {
	pw.out(hashToString(h, 0))
}

// Write an end of line (EOL) marker to the file if it is not on a EOL already.
func (pw *Writer) eol() {
	if curpos := pw.curpos(); curpos != pw.lastEOL {
		fmt.Fprintln(pw.outfile, "")
		pw.lastEOL = curpos
	}
}

func (pw *Writer) out(str string) {
	fmt.Fprintln(pw.outfile, str)
	pw.lastEOL = pw.curpos()
}

// Write a formatted string to the PDF file
func (pw *Writer) outf(format string, str ...interface{}) {
	fmt.Fprintf(pw.outfile, format, str...)
}

// Return the current position in the PDF file. Panics if something is wrong.
func (pw *Writer) curpos() int64 {
	pos, err := pw.outfile.Seek(0, os.SEEK_CUR)
	if err != nil {
		panic(err)
	}
	return pos
}

// Write a start object marker with the next free object.
func (pw *Writer) startObject(onum objectnumber) error {
	var position int64
	position = pw.curpos() + 1
	pw.objectlocations[onum] = position
	pw.outf("\n%d 0 obj\n", onum)
	return nil
}

// Write a simple "endobj" to the PDF file. Return the object number.
func (pw *Writer) endObject() objectnumber {
	onum := pw.nextobject
	pw.eol()
	pw.out("endobj")
	return onum
}
