package ls_qpack_go

/*
#cgo LDFLAGS: ${SRCDIR}/ls-qpack/libls-qpack.a  ${SRCDIR}/ls-qpack/bin/CMakeFiles/interop-encode.dir/__/deps/xxhash/xxhash.c.o
#include <stdlib.h>
#include <string.h>
#include <sys/uio.h>
#include "ls-qpack/include/lsqpack.h"

typedef struct lsqpack_enc lsqpack_enc_t;
typedef struct lsqpack_dec lsqpack_dec_t;

typedef struct {
	lsqpack_dec_t *dec;
	void *base;
	void *orig;
	size_t orig_sz;
	void *stream;
	size_t stream_sz;
	struct lsqpack_header_set *set;
} lsqpack_go_hdr_blk;

void get_ptr_addr(lsqpack_go_hdr_blk *blk, struct lsqpack_header_set ***rec_set, unsigned char ***rec_buf, size_t **stream_sz) {
	*(rec_set) = &(blk->set);
	*(rec_buf) = (unsigned char **) &(blk->base);
	*(stream_sz) = &(blk->stream_sz);
}

void hblock_unblocked(void *hdr_blk) {
	lsqpack_go_hdr_blk *blk = (lsqpack_go_hdr_blk *) hdr_blk;
	size_t remaining_bytes = (blk->orig + blk->orig_sz - blk->base);
	if (remaining_bytes > 0 ){
		int ret = lsqpack_dec_header_read(blk->dec, hdr_blk, (const unsigned char **)&blk->base, remaining_bytes, &blk->set, blk->stream, &blk->stream_sz);
	}
}
*/
import "C"
import (
	"github.com/davecgh/go-spew/spew"
	"unsafe"
)

type Header struct {
	Name, Value string
}
type HeaderBlock struct {
	StreamID uint64
	hdr_blk  *C.lsqpack_go_hdr_blk
	headers []Header
	decoderStream []byte
}

func (hb *HeaderBlock) Headers() []Header {
	if hb.headers == nil && hb.hdr_blk.set != nil && hb.hdr_blk.set.qhs_headers != nil {
		hb.headers = make([]Header, hb.hdr_blk.set.qhs_count)
		h_slice := (*[1 << 30]*C.struct_lsqpack_header)(unsafe.Pointer(hb.hdr_blk.set.qhs_headers))[:hb.hdr_blk.set.qhs_count:hb.hdr_blk.set.qhs_count] // That's how you convert a C array to a Go slice, son
		for i, h := range h_slice {
			hb.headers[i].Name = C.GoStringN(h.qh_name, C.int(h.qh_name_len))
			hb.headers[i].Value = C.GoStringN(h.qh_value, C.int(h.qh_value_len))
		}
		C.free(hb.hdr_blk.orig)
		C.lsqpack_dec_destroy_header_set(hb.hdr_blk.set)
	}

	return hb.headers
}
func (hb *HeaderBlock) DecoderStream() []byte {
	if hb.hdr_blk.stream_sz > 0 {
		hb.decoderStream = C.GoBytes(hb.hdr_blk.stream, C.int(hb.hdr_blk.stream_sz))
		C.free(hb.hdr_blk.stream)
	}
	return hb.decoderStream
}

const (
	LSQPackEncOptServer = 1 << 0
	LSQPackEncOptDup    = 1 << 1
	LSQPackEncOptIxAggr = 1 << 2
	LSQPackEncOptStage2 = 1 << 3
)

type QPackEncoder struct {
	enc *C.lsqpack_enc_t
}

func NewQPackEncoder(server bool) *QPackEncoder {
	q := QPackEncoder{
		enc: (*C.lsqpack_enc_t)(C.malloc(C.sizeof_lsqpack_enc_t)),
	}
	C.lsqpack_enc_preinit(q.enc)
	return &q
}
func (q *QPackEncoder) Init(headerTableSize uint, dynamicTablesize uint, maxRiskedStreams uint, opts uint32) {
	opts |= LSQPackEncOptStage2
	C.lsqpack_enc_init(q.enc, /*SETTINGS_HEADER_TABLE_SIZE*/ C.uint(headerTableSize), C.uint(dynamicTablesize), /*SETTINGS_QPACK_BLOCKED_STREAMS*/ C.uint(maxRiskedStreams), opts)
}
func (q *QPackEncoder) StartHeaderBlock(streamID uint64, seqno uint) bool {
	return C.lsqpack_enc_start_header(q.enc, (C.ulong)(streamID), (C.uint)(seqno)) != 0
}
func (q *QPackEncoder) Encode(name, value string) ([]byte, []byte) {
	var enc_sz C.size_t = 1024
	var header_sz C.size_t = 1024
	var enc_buf [1024]byte
	var header_buf [1024]byte

	name_ptr := C.CString(name)
	value_ptr := C.CString(value)

	ret := C.lsqpack_enc_encode(q.enc,
		(*C.uchar)(unsafe.Pointer(&enc_buf)), (*C.ulong)(unsafe.Pointer(&enc_sz)),
		(*C.uchar)(unsafe.Pointer(&header_buf)), (*C.ulong)(unsafe.Pointer(&header_sz)),
		(*C.char)(name_ptr), (C.uint)(len(name)),
		(*C.char)(value_ptr), (C.uint)(len(value)),
		0)

	C.free((unsafe.Pointer)(name_ptr))
	C.free((unsafe.Pointer)(value_ptr))

	if ret != C.LQES_OK {
		println("lsqpack_enc_encode returned error code:", ret)
		return nil, nil
	}
	return enc_buf[:enc_sz], header_buf[:header_sz]
}
func (q *QPackEncoder) EndHeaderBlock() []byte {
	header_buf := make([]byte, C.lsqpack_enc_header_data_prefix_size(q.enc))
	header_ptr := C.CBytes(header_buf)
	defer C.free(header_ptr)

	ret := (int)(C.lsqpack_enc_end_header(q.enc, (*C.uchar)(header_ptr), (C.ulong)(len(header_buf))))

	if ret <= 0 {
		println("lsqpack_enc_end_header returned error code:", ret)
		return nil
	}

	header_data_prefix := C.GoBytes(header_ptr, (C.int)(ret))
	return header_data_prefix
}
func (q *QPackEncoder) DecoderIn(data []byte) bool {
	data_ptr := C.CBytes(data)
	defer C.free(data_ptr)

	return C.lsqpack_enc_decoder_in(q.enc, (*C.uchar)(data_ptr), C.ulong(len(data))) != 0
}

type QPackDecoder struct {
	dec                 *C.struct_lsqpack_dec
	pendingHeaderBlocks []HeaderBlock
}

func NewQPackDecoder(dynamicTablesize uint, qpackBlockedStreams uint, ) *QPackDecoder {
	q := QPackDecoder{
		dec: (*C.lsqpack_dec_t)(C.malloc(C.sizeof_lsqpack_dec_t)),
	}
	C.lsqpack_dec_init(q.dec, C.uint(dynamicTablesize), C.uint(qpackBlockedStreams), (*[0]byte)(C.hblock_unblocked))
	return &q
}
func (q *QPackDecoder) HeaderIn(headerBuf []byte, streamID uint64) int {
	var hdr_blk C.lsqpack_go_hdr_blk
	header_ptr := C.CBytes(headerBuf)
	hdr_blk.dec = q.dec
	hdr_blk.base = header_ptr
	hdr_blk.orig = header_ptr
	hdr_blk.orig_sz = (C.ulong)(len(headerBuf))
	hdr_blk.set = (*C.struct_lsqpack_header_set)(C.malloc(C.sizeof_struct_lsqpack_header_set))
	C.memset(unsafe.Pointer(hdr_blk.set), 0, C.sizeof_struct_lsqpack_header_set)

	hdr_blk.stream = C.malloc(C.LSQPACK_LONGEST_HACK)
	hdr_blk.stream_sz = 0

	q.pendingHeaderBlocks = append(q.pendingHeaderBlocks, HeaderBlock{StreamID: streamID, hdr_blk: &hdr_blk})

	var set_ptr **C.struct_lsqpack_header_set
	var buf_ptr **C.uchar
	var stream_sz_ptr *C.size_t

	C.get_ptr_addr(&hdr_blk, &set_ptr, &buf_ptr, &stream_sz_ptr)

	if ret := C.lsqpack_dec_header_in(q.dec, unsafe.Pointer(&hdr_blk), (C.ulong)(streamID), hdr_blk.orig_sz, buf_ptr, C.ulong(len(headerBuf)), set_ptr, (*C.uchar)(hdr_blk.stream), stream_sz_ptr); ret > 1 {
		println("lsqpack_dec_header_in returned error code:", ret)
		q.Error()
	}
	return int(uintptr(hdr_blk.base) - uintptr(hdr_blk.orig))
}
func (q *QPackDecoder) EncoderIn(in []byte) bool {
	in_ptr := C.CBytes(in)
	//defer C.free(in_ptr)
	return C.lsqpack_dec_enc_in(q.dec, (*C.uchar)(in_ptr), (C.ulong)(len(in))) != 0
}
func (q *QPackDecoder) Error() {
	spew.Dump(C.lsqpack_dec_get_err_info(q.dec))
}
func (q *QPackDecoder) DecodedHeaderBlocks() []HeaderBlock {
	var hbs []HeaderBlock
	pHbs := make([]HeaderBlock, len(hbs))
	for _, hb := range q.pendingHeaderBlocks {
		if hb.hdr_blk.set.qhs_headers != nil {
			hbs = append(hbs, hb)
		} else {
			pHbs = append(pHbs, hb)
		}
	}
	q.pendingHeaderBlocks = pHbs
	return hbs
}