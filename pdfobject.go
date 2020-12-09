package pdf

import (
	"bytes"
)

// PDFObject has information about a specific PDF object
type PDFObject struct {
	ObjectNumber objectnumber
	Data         bytes.Buffer
	pdfwriter    *Writer
}

// NewObject create a new PDF object and reserves an object
// number for it.
// The object is not written to the PDF until Save() is called.
func (pw *Writer) NewPDFObject() *PDFObject {
	obj := &PDFObject{}
	obj.ObjectNumber = pw.nextObject()
	obj.pdfwriter = pw
	return obj
}

// Save adds the PDF object to the main PDF file.
func (obj *PDFObject) Save() {
	obj.pdfwriter.startObject(obj.ObjectNumber)
	obj.Data.WriteTo(obj.pdfwriter.outfile)
	obj.pdfwriter.endObject()
}

// Dict writes the dict d to a PDF object
func (obj *PDFObject) Dict(d Dict) {
	obj.Data.WriteString(hashToString(d, 0))
}
