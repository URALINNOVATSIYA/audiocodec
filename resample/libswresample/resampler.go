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
	"os"
	"time"
	"unsafe"

	"github.com/URALINNOVATSIYA/audiocodec"
	"github.com/URALINNOVATSIYA/audiocodec/resample"
)

type Params struct {
	IncomingCodec          *audiocodec.Codec
	OutgoingCodec          *audiocodec.Codec
	OutgoingBufferDuration time.Duration
}

type Resampler[P Params] struct {
	swrContext      *C.SwrContext
	incomingPointer **C.uint8_t
	outgoingPointer **C.uint8_t

	incomingCodec  *audiocodec.Codec
	outgoingCodec  *audiocodec.Codec
	outgoingBuffer []byte
	debug          bool
	incomingAudio  *audiocodec.Wav
	outgoingAudio  *audiocodec.Wav

	params P
}

type Pool = resample.Pool[*Resampler[Params], Params]

func NewPool() *Pool {
	ctor := func(params Params) (*Resampler[Params], error) {
		return NewResampler(params.IncomingCodec, params.OutgoingCodec, params.OutgoingBufferDuration)
	}

	return resample.NewPool(ctor)
}

func NewResampler(incomingCodec *audiocodec.Codec, outgoingCodec *audiocodec.Codec, outgoingBufferDuration time.Duration) (*Resampler[Params], error) {
	if !incomingCodec.IsPcm() || !outgoingCodec.IsPcm() {
		return nil, audiocodec.NotPcm
	}

	if incomingCodec.IsEqual(outgoingCodec) {
		return nil, audiocodec.IncomingAndOutgoingCodecsIsEquals
	}

	params := Params{
		IncomingCodec:          incomingCodec,
		OutgoingCodec:          outgoingCodec,
		OutgoingBufferDuration: outgoingBufferDuration,
	}

	r := &Resampler[Params]{
		swrContext:      C.swr_alloc(),
		incomingPointer: (**C.uint8_t)(C.malloc(C.size_t(unsafe.Sizeof((*C.uint8_t)(nil))))),
		outgoingPointer: (**C.uint8_t)(C.malloc(C.size_t(unsafe.Sizeof((*C.uint8_t)(nil))))),
		incomingCodec:   incomingCodec,
		outgoingCodec:   outgoingCodec,
		outgoingBuffer:  make([]byte, outgoingCodec.Size(outgoingBufferDuration)),
		params:          params,
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

func (r *Resampler[P]) sampleFormat(codec *audiocodec.Codec) (int32, error) {
	switch codec.BitRate {
	case 16:
		return C.AV_SAMPLE_FMT_S16, nil
	case 32:
		return C.AV_SAMPLE_FMT_S32, nil
	default:
		return 0, fmt.Errorf("not supported bit rate: %d", codec.BitRate)
	}
}

func (r *Resampler[P]) Resample(incomingFrame []byte) ([]byte, error) {
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

func (r *Resampler[P]) Flush() ([]byte, error) {
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

func (r *Resampler[P]) Free() error {
	C.swr_close((*C.SwrContext)(unsafe.Pointer(r.swrContext)))
	C.swr_free(&r.swrContext)
	C.free(unsafe.Pointer(r.incomingPointer))
	C.free(unsafe.Pointer(r.outgoingPointer))

	r.swrContext = nil
	r.incomingPointer = nil
	r.outgoingPointer = nil

	return nil
}

func (r *Resampler[P]) DebugEnable() {
	r.debug = true
	r.incomingAudio = audiocodec.NewWav(r.incomingCodec)
	r.outgoingAudio = audiocodec.NewWav(r.outgoingCodec)
}

func (r *Resampler[P]) DebugDisable() {
	r.debug = false
	r.incomingAudio = nil
	r.outgoingAudio = nil
}

func (r *Resampler[P]) SaveIncomingAudio(fileName string) (int64, error) {
	return r.saveAudio(fileName, r.incomingAudio)
}

func (r *Resampler[P]) SaveOutgoingAudio(fileName string) (int64, error) {
	return r.saveAudio(fileName, r.outgoingAudio)
}

func (r *Resampler[P]) saveAudio(fileName string, wav *audiocodec.Wav) (int64, error) {
	file, err := os.Create(fileName)
	if err != nil {
		return 0, err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	return wav.WriteTo(file)
}

func (r *Resampler[P]) Params() P {
	return r.params
}

func (r *Resampler[P]) Reset() error { return nil }
