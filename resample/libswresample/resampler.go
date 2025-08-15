package libswresample

/*
	#cgo pkg-config: libswresample
	#cgo pkg-config: libavutil

	#include <libswresample/swresample.h>
	#include <libavutil/opt.h>
	#include <libavutil/samplefmt.h>
*/
import "C"
import (
	"fmt"
	"github.com/URALINNOVATSIYA/audiocodec"
	"os"
	"time"
	"unsafe"
)

type Resampler struct {
	swrContext      *C.SwrContext
	incomingPointer **C.uint8_t
	outgoingPointer **C.uint8_t

	incomingCodec  *codec.Codec
	outgoingCodec  *codec.Codec
	outgoingBuffer []byte
	debug          bool
	incomingAudio  *codec.Wav
	outgoingAudio  *codec.Wav
}

func NewResampler(incomingCodec *codec.Codec, outgoingCodec *codec.Codec, outgoingBufferDuration time.Duration) (*Resampler, error) {
	if !incomingCodec.IsPcm() || !outgoingCodec.IsPcm() {
		return nil, codec.NotPcm
	}

	if incomingCodec.IsEqual(outgoingCodec) {
		return nil, codec.IncomingAndOutgoingCodecsIsEquals
	}

	r := &Resampler{
		swrContext:      C.swr_alloc(),
		incomingPointer: (**C.uint8_t)(C.malloc(C.size_t(unsafe.Sizeof((*C.uint8_t)(nil))))),
		outgoingPointer: (**C.uint8_t)(C.malloc(C.size_t(unsafe.Sizeof((*C.uint8_t)(nil))))),
		incomingCodec:   incomingCodec,
		outgoingCodec:   outgoingCodec,
		outgoingBuffer:  make([]byte, outgoingCodec.Size(outgoingBufferDuration)),
	}

	C.av_opt_set_channel_layout(unsafe.Pointer(r.swrContext), C.CString("in_channel_layout"), C.AV_CH_LAYOUT_MONO, 0)
	C.av_opt_set_channel_layout(unsafe.Pointer(r.swrContext), C.CString("out_channel_layout"), C.AV_CH_LAYOUT_MONO, 0)
	C.av_opt_set_int(unsafe.Pointer(r.swrContext), C.CString("in_sample_rate"), C.int64_t(incomingCodec.SampleRate), 0)
	C.av_opt_set_int(unsafe.Pointer(r.swrContext), C.CString("out_sample_rate"), C.int64_t(outgoingCodec.SampleRate), 0)

	sampleFormat, err := r.sampleFormat(incomingCodec)
	if err != nil {
		return nil, err
	}
	C.av_opt_set_sample_fmt(unsafe.Pointer(r.swrContext), C.CString("in_sample_fmt"), sampleFormat, 0)

	sampleFormat, err = r.sampleFormat(outgoingCodec)
	if err != nil {
		return nil, err
	}
	C.av_opt_set_sample_fmt(unsafe.Pointer(r.swrContext), C.CString("out_sample_fmt"), sampleFormat, 0)

	C.swr_init(r.swrContext)

	return r, nil
}

func (r *Resampler) sampleFormat(codec *codec.Codec) (int32, error) {
	switch codec.BitRate {
	case 16:
		return C.AV_SAMPLE_FMT_S16, nil
	case 32:
		return C.AV_SAMPLE_FMT_S32, nil
	default:
		return 0, fmt.Errorf("not supported bit rate: %d", codec.BitRate)
	}
}

func (r *Resampler) Resample(incomingFrame []byte) ([]byte, error) {
	*r.incomingPointer = (*C.uint8_t)(&incomingFrame[0])
	*r.outgoingPointer = (*C.uint8_t)(&r.outgoingBuffer[0])

	result := int(C.swr_convert(
		r.swrContext,
		r.outgoingPointer,
		C.int(r.outgoingCodec.SampleCountBySize(len(r.outgoingBuffer))),
		r.incomingPointer,
		C.int(r.incomingCodec.SampleCountBySize(len(incomingFrame))),
	))
	if result < 0 {
		return nil, fmt.Errorf("swr_convert failed")
	}

	if r.debug {
		r.incomingAudio.Write(incomingFrame)
		r.outgoingAudio.Write(r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(result)])
	}

	return r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(result)], nil
}

func (r *Resampler) Flush() ([]byte, error) {
	*r.outgoingPointer = (*C.uint8_t)(&r.outgoingBuffer[0])

	result := int(C.swr_convert(
		r.swrContext,
		r.outgoingPointer,
		C.int(r.outgoingCodec.SampleCountBySize(len(r.outgoingBuffer))),
		nil,
		0,
	))
	if result < 0 {
		return nil, fmt.Errorf("swr_convert failed")
	}

	if r.debug {
		r.outgoingAudio.Write(r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(result)])
	}

	return r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(result)], nil
}

func (r *Resampler) Free() {
	C.swr_close((*C.SwrContext)(unsafe.Pointer(r.swrContext)))
	C.swr_free(&r.swrContext)
	C.free(unsafe.Pointer(r.incomingPointer))
	C.free(unsafe.Pointer(r.outgoingPointer))

	r.swrContext = nil
	r.incomingPointer = nil
	r.outgoingPointer = nil
}

func (r *Resampler) DebugEnable() {
	r.debug = true
	r.incomingAudio = codec.NewWav(r.incomingCodec)
	r.outgoingAudio = codec.NewWav(r.outgoingCodec)
}

func (r *Resampler) DebugDisable() {
	r.debug = false
	r.incomingAudio = nil
	r.outgoingAudio = nil
}

func (r *Resampler) SaveIncomingAudio(fileName string) (int, error) {
	return r.saveAudio(fileName, r.incomingAudio)
}

func (r *Resampler) SaveOutgoingAudio(fileName string) (int, error) {
	return r.saveAudio(fileName, r.outgoingAudio)
}

func (r *Resampler) saveAudio(fileName string, wav *codec.Wav) (int, error) {
	file, err := os.Create(fileName)
	if err != nil {
		return 0, err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	return wav.ExportTo(file)
}
