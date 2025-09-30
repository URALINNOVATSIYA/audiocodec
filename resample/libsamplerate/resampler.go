package libsamplerate

/*
#cgo pkg-config: samplerate

#include <samplerate.h>
#include <stdlib.h>

SRC_DATA *alloc_src_data(float *data_in, float *data_out, long output_frames, double src_ratio) {
	SRC_DATA *src_data = malloc(sizeof(SRC_DATA));
    src_data->data_in = data_in;
    src_data->data_out = data_out;
    src_data->output_frames = output_frames;
    src_data->src_ratio = src_ratio;
	return src_data;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"github.com/URALINNOVATSIYA/audiocodec"
	"github.com/URALINNOVATSIYA/audiocodec/binary"
	"os"
	"time"
	"unsafe"
)

type ConverterType C.int

// SincBestQuality - This is a bandlimited interpolator derived from the mathematical sinc function and this is the
// highest quality sinc based converter, providing a worst case Signal-to-Noise Ratio (SNR) of 97 decibels (dB) at a
// bandwidth of 97%. All three SRC_SINC_* converters are based on the techniques of Julius O. Smith although this
// code was developed independantly.
// SincMediumQuality - This is another bandlimited interpolator much like the previous one. It has an SNR of 97dB
// and a bandwidth of 90%. The speed of the conversion is much faster than the previous one.
// SincFastest - This is the fastest bandlimited interpolator and has an SNR of 97dB and a bandwidth of 80%.
// ZeroOrderHold - A Zero Order Hold converter (interpolated value is equal to the last value). The quality is poor
// but the conversion speed is blindlingly fast.
// Linear - A linear converter. Again the quality is poor, but the conversion speed is blindingly fast.
const (
	SincBestQuality   ConverterType = C.SRC_SINC_BEST_QUALITY
	SincMediumQuality ConverterType = C.SRC_SINC_MEDIUM_QUALITY
	SincFastest       ConverterType = C.SRC_SINC_FASTEST
	ZeroOrderHold     ConverterType = C.SRC_ZERO_ORDER_HOLD
	Linear            ConverterType = C.SRC_LINEAR
)

type Resampler struct {
	srcState       *C.SRC_STATE
	srcData        *C.SRC_DATA
	incomingBuffer []C.float
	outgoingBuffer []C.float

	incomingCodec      *audiocodec.Codec
	incomingSampleSize int
	outgoingCodec      *audiocodec.Codec
	outgoingSampleSize int
	encoder            func(sample float32, buf []byte)
	decoder            func(sample []byte) float32
	outgoingBufferGo   []byte
	debug              bool
	incomingAudio      *audiocodec.Wav
	outgoingAudio      *audiocodec.Wav
}

func NewResampler(incomingCodec *audiocodec.Codec, outgoingCodec *audiocodec.Codec, frameDuration time.Duration, converterType ConverterType) (*Resampler, error) {
	if !incomingCodec.IsPcm() || !outgoingCodec.IsPcm() {
		return nil, audiocodec.NotPcm
	}

	if incomingCodec.IsEqual(outgoingCodec) {
		return nil, audiocodec.IncomingAndOutgoingCodecsIsEquals
	}

	resampler := &Resampler{
		incomingCodec:      incomingCodec,
		incomingSampleSize: incomingCodec.SampleSize(),
		outgoingCodec:      outgoingCodec,
		outgoingSampleSize: outgoingCodec.SampleSize(),
		outgoingBufferGo:   make([]byte, outgoingCodec.Size(frameDuration)),
	}

	switch incomingCodec.BitRate {
	case 32:
		resampler.decoder = binary.Bytes32bitToFloat32
	case 16:
		resampler.decoder = binary.Bytes16bitToFloat32
	default:
		return nil, fmt.Errorf("not supported bitrate: %d.", incomingCodec.BitRate)
	}

	switch outgoingCodec.BitRate {
	case 32:
		resampler.encoder = binary.Float32ToBytes32bit
	case 16:
		resampler.encoder = binary.Float32ToBytes16bit
	default:
		return nil, fmt.Errorf("not supported bitrate: %d.", outgoingCodec.BitRate)
	}

	var cErr *C.int
	resampler.srcState = C.src_new(C.int(converterType), C.int(1), cErr)
	if resampler.srcState == nil {
		return nil, errors.New("could not initialize libsamplerate converter.")
	}

	resampler.incomingBuffer = make([]C.float, incomingCodec.SampleCountByDuration(frameDuration))
	resampler.outgoingBuffer = make([]C.float, outgoingCodec.SampleCountByDuration(frameDuration))

	resampler.srcData = C.alloc_src_data(
		&resampler.incomingBuffer[0],
		&resampler.outgoingBuffer[0],
		C.long(len(resampler.outgoingBuffer)),
		C.double(float64(outgoingCodec.SampleRate)/float64(incomingCodec.SampleRate)),
	)

	return resampler, nil
}

func (r *Resampler) Free() error {
	C.free(unsafe.Pointer(r.srcData))
	if srcState := C.src_delete(r.srcState); srcState != nil {
		return errors.New("could not free resampler.")
	}
	return nil
}

// Resample Метод для обработки потоковых данных в бинарном формате
// http://www.mega-nerd.com/SRC/api_full.html
func (r *Resampler) Resample(incomingFrame []byte, final bool) ([]byte, error) {
	if len(incomingFrame) > len(r.incomingBuffer)*r.incomingSampleSize {
		return nil, errors.New("incoming frame is larger than incoming buffer")
	}

	sampleId := 0
	for i := 0; i < len(incomingFrame); i += r.incomingSampleSize {
		r.incomingBuffer[sampleId] = C.float(r.decoder(incomingFrame[i : i+r.incomingSampleSize]))
		sampleId++
	}

	var isFinalFrame C.int
	if final {
		isFinalFrame = C.int(1)
	} else {
		isFinalFrame = C.int(0)
	}

	r.srcData.input_frames = C.long(sampleId)
	r.srcData.end_of_input = isFinalFrame

	processErr := C.src_process(r.srcState, r.srcData)
	if processErr != 0 {
		return nil, fmt.Errorf("error code: %d; %s", int(processErr), r.error(processErr))
	}

	outgoingSampleCount := int(r.srcData.output_frames_gen)
	sampleId = 0
	for i := 0; sampleId < outgoingSampleCount; i += r.outgoingSampleSize {
		r.encoder(float32(r.outgoingBuffer[sampleId]), r.outgoingBufferGo[i:i+r.outgoingSampleSize])
		sampleId++
	}

	if r.debug {
		r.incomingAudio.Write(incomingFrame)
		r.outgoingAudio.Write(r.outgoingBufferGo[:r.outgoingCodec.SizeBySampleCount(outgoingSampleCount)])
	}

	return r.outgoingBufferGo[:r.outgoingCodec.SizeBySampleCount(outgoingSampleCount)], nil
}

func (r *Resampler) Reset() error {
	if err := C.src_reset(r.srcState); err != 0 {
		return fmt.Errorf("error code: %d; %s", int(err), r.error(err))
	}
	return nil
}

func (r *Resampler) error(errCode C.int) string {
	return C.GoString(C.src_strerror(errCode))
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
