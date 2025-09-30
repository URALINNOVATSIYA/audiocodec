package soxr

/*
#cgo pkg-config: soxr

#include <soxr.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"github.com/URALINNOVATSIYA/audiocodec"
	"os"
	"time"
)

type Quality C.int

const (
	Quick           Quality = C.SOXR_QQ  // Quick cubic interpolation
	LowQuality      Quality = C.SOXR_LQ  // LowQuality 16-bit with larger rolloff
	MediumQuality   Quality = C.SOXR_MQ  // MediumQuality 16-bit with medium rolloff
	HighQuality     Quality = C.SOXR_HQ  // HighQuality high quality
	VeryHighQuality Quality = C.SOXR_VHQ // VeryHighQuality very high quality
)

type Resampler struct {
	soxr         C.soxr_t
	incomingUsed C.size_t
	outgoingUsed C.size_t
	soxErr       C.soxr_error_t

	incomingCodec  *audiocodec.Codec
	outgoingCodec  *audiocodec.Codec
	outgoingBuffer []byte
	debug          bool
	incomingAudio  *audiocodec.Wav
	outgoingAudio  *audiocodec.Wav
}

func NewResampler(incomingCodec *audiocodec.Codec, outgoingCodec *audiocodec.Codec, outgoingBufferDuration time.Duration, quality Quality) (*Resampler, error) {
	if !incomingCodec.IsPcm() || !outgoingCodec.IsPcm() {
		return nil, audiocodec.NotPcm
	}

	if incomingCodec.IsEqual(outgoingCodec) {
		return nil, audiocodec.IncomingAndOutgoingCodecsIsEquals
	}

	resampler := &Resampler{
		incomingCodec:  incomingCodec,
		outgoingCodec:  outgoingCodec,
		outgoingBuffer: make([]byte, outgoingCodec.Size(outgoingBufferDuration)),
	}

	var err error
	var incomingDataType C.soxr_datatype_t
	incomingDataType, err = resampler.dataType(incomingCodec)
	if err != nil {
		return nil, err
	}

	var outgoingDataType C.soxr_datatype_t
	outgoingDataType, err = resampler.dataType(outgoingCodec)
	if err != nil {
		return nil, err
	}

	ioSpec := C.soxr_io_spec(incomingDataType, outgoingDataType)
	qualitySpec := C.soxr_quality_spec(C.ulong(quality), 0)
	runtimeSpec := C.soxr_runtime_spec(C.uint(1))

	resampler.soxr = C.soxr_create(
		C.double(incomingCodec.SampleRate),
		C.double(outgoingCodec.SampleRate),
		C.uint(1),
		&resampler.soxErr,
		&ioSpec,
		&qualitySpec,
		&runtimeSpec,
	)
	if err = resampler.error(); err != nil {
		return nil, err
	}

	return resampler, nil
}

func (r *Resampler) error() error {
	soxrErrStr := C.GoString(r.soxErr)
	if soxrErrStr != "" && soxrErrStr != "0" {
		return errors.New(soxrErrStr)
	}
	return nil
}

func (r *Resampler) dataType(codec *audiocodec.Codec) (C.soxr_datatype_t, error) {
	switch codec.BitRate {
	case 16:
		return C.SOXR_INT16, nil
	case 32:
		return C.SOXR_FLOAT32, nil
	default:
		return 0, fmt.Errorf("not supported bit rate: %d", codec.BitRate)
	}
}

func (r *Resampler) Resample(incomingFrame []byte) ([]byte, error) {
	r.soxErr = C.soxr_process(
		r.soxr,
		C.soxr_in_t(&incomingFrame[0]),
		C.size_t(r.incomingCodec.SampleCountBySize(len(incomingFrame))),
		&r.incomingUsed,
		C.soxr_out_t(&r.outgoingBuffer[0]),
		C.size_t(r.outgoingCodec.SampleCountBySize(len(r.outgoingBuffer))),
		&r.outgoingUsed,
	)
	if err := r.error(); err != nil {
		return nil, err
	}

	if r.debug {
		_, _ = r.incomingAudio.Write(incomingFrame)
		_, _ = r.outgoingAudio.Write(r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(int(r.outgoingUsed))])
	}

	return r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(int(r.outgoingUsed))], nil
}

func (r *Resampler) Flush() ([]byte, error) {
	r.soxErr = C.soxr_process(
		r.soxr,
		nil,
		0,
		nil,
		C.soxr_out_t(&r.outgoingBuffer[0]),
		C.size_t(r.outgoingCodec.SampleCountBySize(len(r.outgoingBuffer))),
		&r.outgoingUsed,
	)
	if err := r.error(); err != nil {
		return nil, err
	}

	if r.debug {
		_, _ = r.outgoingAudio.Write(r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(int(r.outgoingUsed))])
	}

	return r.outgoingBuffer[:r.outgoingCodec.SizeBySampleCount(int(r.outgoingUsed))], nil
}

func (r *Resampler) Free() error {
	r.soxErr = C.soxr_clear(r.soxr)
	if err := r.error(); err != nil {
		return err
	}
	C.soxr_delete(r.soxr)

	return nil
}

func (r *Resampler) Reset() error {
	r.soxErr = C.soxr_clear(r.soxr)
	if err := r.error(); err != nil {
		return err
	}

	return nil
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
