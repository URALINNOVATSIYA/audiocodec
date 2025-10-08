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

const maxResampleFrameDuration = 100 * time.Millisecond

type Resampler struct {
	swrContext      *C.SwrContext
	incomingPointer **C.uint8_t
	outgoingPointer **C.uint8_t

	incomingCodec  *audiocodec.Codec
	outgoingCodec  *audiocodec.Codec
	outgoingBuffer []byte
	debug          bool
	incomingAudio  *audiocodec.Wav
	outgoingAudio  *audiocodec.Wav
}

func NewResampler(incomingCodec *audiocodec.Codec, outgoingCodec *audiocodec.Codec) (*Resampler, error) {
	if !incomingCodec.IsPcm() || !outgoingCodec.IsPcm() {
		return nil, audiocodec.NotPcm
	}

	if incomingCodec.IsEqual(outgoingCodec) {
		return nil, audiocodec.IncomingAndOutgoingCodecsIsEquals
	}

	r := &Resampler{
		swrContext:      C.swr_alloc(),
		incomingPointer: (**C.uint8_t)(C.malloc(C.size_t(unsafe.Sizeof((*C.uint8_t)(nil))))),
		outgoingPointer: (**C.uint8_t)(C.malloc(C.size_t(unsafe.Sizeof((*C.uint8_t)(nil))))),
		incomingCodec:   incomingCodec,
		outgoingCodec:   outgoingCodec,
		outgoingBuffer:  make([]byte, outgoingCodec.Size(maxResampleFrameDuration)),
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

func (r *Resampler) sampleFormat(codec *audiocodec.Codec) (int32, error) {
	switch codec.BitRate {
	case 16:
		return C.AV_SAMPLE_FMT_S16, nil
	case 32:
		return C.AV_SAMPLE_FMT_S32, nil
	default:
		return 0, fmt.Errorf("not supported bit rate: %d", codec.BitRate)
	}
}

func (r *Resampler) Resample(incomingData []byte) ([]byte, error) {
	incomingDataSize := len(incomingData)
	outgoingData := make([]byte, r.outgoingCodec.Size(r.incomingCodec.Duration(incomingDataSize))+r.delaySize())
	maxResampleFrameSize := r.incomingCodec.Size(maxResampleFrameDuration)

	var outgoingDataPos int
	var chunk []byte
	var err error
	for pos := 0; pos < incomingDataSize; pos += maxResampleFrameSize {
		to := pos + maxResampleFrameSize
		if to > incomingDataSize {
			chunk, err = r.resample(incomingData[pos:])
		} else {
			chunk, err = r.resample(incomingData[pos:to])
		}

		if err != nil {
			return nil, err
		}

		chunkLen := len(chunk)
		if chunkLen > 0 {
			copy(outgoingData[outgoingDataPos:outgoingDataPos+chunkLen], chunk)
			outgoingDataPos += chunkLen
		}
	}

	return outgoingData[:outgoingDataPos], nil
}

func (r *Resampler) resample(incomingFrame []byte) ([]byte, error) {
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
	outgoingData := make([]byte, r.delaySize())
	outgoingDataPos := 0

	if len(outgoingData) == 0 {
		return nil, nil
	}

	for {
		chunk, err := r.flush()
		if err != nil {
			return nil, err
		}

		chunkLen := len(chunk)
		if chunkLen > 0 {
			copy(outgoingData[outgoingDataPos:outgoingDataPos+chunkLen], chunk)
			outgoingDataPos += chunkLen
		}

		if len(chunk) < len(r.outgoingBuffer) {
			break
		}
	}

	return outgoingData[:outgoingDataPos], nil
}

func (r *Resampler) flush() ([]byte, error) {
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

func (r *Resampler) delaySize() int {
	return r.outgoingCodec.Size(time.Duration(int64(C.swr_get_delay(r.swrContext, C.int64_t(1000)))) * time.Millisecond)
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
	r.incomingAudio = audiocodec.NewWav(r.incomingCodec)
	r.outgoingAudio = audiocodec.NewWav(r.outgoingCodec)
}

func (r *Resampler) DebugDisable() {
	r.debug = false
	r.incomingAudio = nil
	r.outgoingAudio = nil
}

func (r *Resampler) SaveIncomingAudio(fileName string) (int64, error) {
	return r.saveAudio(fileName, r.incomingAudio)
}

func (r *Resampler) SaveOutgoingAudio(fileName string) (int64, error) {
	return r.saveAudio(fileName, r.outgoingAudio)
}

func (r *Resampler) saveAudio(fileName string, wav *audiocodec.Wav) (int64, error) {
	file, err := os.Create(fileName)
	if err != nil {
		return 0, err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	return wav.WriteTo(file)
}

func (r *Resampler) Reset() error {
	for {
		chunk, err := r.flush()
		if err != nil {
			return err
		}

		if len(chunk) < len(r.outgoingBuffer) {
			break
		}
	}

	return nil
}

func (r *Resampler) StreamResample(incomingCh <-chan []byte) <-chan []byte {
	outgoingCh := make(chan []byte)

	go func() {
		defer close(outgoingCh)

		for incomingChunk := range incomingCh {
			if len(incomingChunk) == 0 {
				continue
			}

			outgoingChunk, err := r.Resample(incomingChunk)
			if err != nil {
				return
			}

			if len(outgoingChunk) == 0 {
				continue
			}

			outgoingCh <- outgoingChunk
		}

		outgoingChunk, err := r.Flush()
		if err != nil {
			return
		}

		if len(outgoingChunk) == 0 {
			return
		}

		outgoingCh <- outgoingChunk
	}()

	return outgoingCh
}

func (r *Resampler) IncomingCodec() *audiocodec.Codec {
	return r.incomingCodec
}

func (r *Resampler) OutgoingCodec() *audiocodec.Codec {
	return r.outgoingCodec
}
