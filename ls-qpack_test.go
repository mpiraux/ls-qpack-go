package ls_qpack_go

import (
	"github.com/davecgh/go-spew/spew"
	"testing"
)

func TestBasicEncoder(t *testing.T) {
	e := NewQPackEncoder(false)

	headers := []Header{Header{":method", "GET"}, Header{":path", "/"}, Header{"Host", "example.org"}, Header{"User-Agent", "curl/7.59.0"}, Header{"Accept", "*/*"}}

	if e.StartHeaderBlock(4, 0) {
		t.Fatal("Failed to start header block")
	}

	var hb []byte

	for _, h := range headers {
		enc, hdb := e.Encode(h.Name, h.Value)
		if len(enc) > 0 {
			t.Error("Encoder stream should not be used")
		}
		hb = append(hb, hdb...)
	}

	hb = append(e.EndHeaderBlock(), hb...)

	d := NewQPackDecoder(1024, 100)
	if ret := d.HeaderIn(hb, 4); ret != len(hb) {
		t.Fatal("Not all input was consumed, expected ", len(hb), " got ", ret)
	}

	blocks := d.DecodedHeaderBlocks()
	if len(blocks) != 1 {
		spew.Dump(blocks)
		t.Fatal("Only one block should be decoded")
	}
	b := blocks[0]
	if b.StreamID != 4 {
		t.Fatal("Wrong stream ID returned in the header block")
	}

	if len(b.Headers()) < len(headers) {
		t.Fatal("Number of headers encoded and decoded does not match")
	}

	for i, h := range b.Headers() {
		if h != headers[i] {
			spew.Dump(h, headers[i])
			t.Error("Header before and after decoding do not match")
		}
	}
}

func TestEncoderStream(t *testing.T) {
	e := NewQPackEncoder(false)
	e.Init(4096, 1024, 100, LSQPackEncOptIxAggr)

	headerName := "VeryCommonHeader"
	headerValue := "42"

	e.StartHeaderBlock(4, 1)
	encStream, hb := e.Encode(headerName, headerValue)
	hdp := e.EndHeaderBlock()

	spew.Dump(encStream, hb, hdp)

	d := NewQPackDecoder(1024,100)
	if ret := d.HeaderIn(append(hdp, hb...), 4); ret == len(hdp) + len(hb) {
		t.Fatal("Decoder should be blocked")
	}
	if len(d.DecodedHeaderBlocks()) != 0 {
		t.Fatal("No decoded hbs should be available until encoder stream is fed")
	}

	if d.EncoderIn(encStream) {
		t.Fatal("EncoderIn should not fail")
	}

	hbs := d.DecodedHeaderBlocks()
	if len(hbs) != 1 {
		t.Fatal("One header block should be decoded")
	}
	headers := hbs[0].Headers()
	if len(headers) != 1 {
		spew.Dump(headers)
		t.Fatal("One header should be present in the header block")
	}
	if headers[0].Name != headerName || headers[0].Value != headerValue {
		spew.Dump(headers[0])
		t.Fatal("Decoded header does not match")
	}
	spew.Dump(hbs[0].DecoderStream())
}
