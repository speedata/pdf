package pdf

type PDFStream struct {
	data []byte
	dict Dict
}

func NewPDFStream(data []byte) *PDFStream {
	s := PDFStream{}
	s.data = data
	s.dict = make(Dict)
	return &s
}
