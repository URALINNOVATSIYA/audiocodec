package soxr

import (
	"sync"

	"github.com/URALINNOVATSIYA/audiocodec"
)

type Pool struct {
	mu      *sync.RWMutex
	items   map[int64][]*Resampler
	quality Quality
}

func NewPool(
	quality Quality,
) *Pool {

	return &Pool{
		mu:      &sync.RWMutex{},
		quality: quality,
		items:   make(map[int64][]*Resampler),
	}
}

func (p *Pool) Get(
	inSampleRate int,
	inBitRate int,
	outSampleRate int,
	outBitRate int,
) (*Resampler, error) {

	return p.GetByCodecs(
		audiocodec.NewPcmCodec(inSampleRate, inBitRate),
		audiocodec.NewPcmCodec(outSampleRate, outBitRate),
	)
}

func (p *Pool) GetByCodecs(
	in *audiocodec.Codec,
	out *audiocodec.Codec,
) (*Resampler, error) {

	p.mu.Lock()
	defer p.mu.Unlock()

	key := hash(in.SampleRate, in.BitRate, out.SampleRate, out.BitRate)

	resamplers, contains := p.items[key]
	if !contains {
		resamplers = make([]*Resampler, 0, 8)

		p.items[key] = resamplers
	}

	if len(resamplers) == 0 {
		return NewResampler(in, out, p.quality)
	}

	lastIndex := len(resamplers) - 1

	resampler := resamplers[lastIndex]

	if err := resampler.Reset(); err != nil {
		return nil, err
	}

	resamplers[lastIndex] = nil
	resamplers = resamplers[:lastIndex]

	p.items[key] = resamplers

	return resampler, nil
}

func (p *Pool) Put(
	resampler *Resampler,
) {

	p.mu.Lock()
	defer p.mu.Unlock()

	key := hash(
		resampler.incomingCodec.SampleRate,
		resampler.incomingCodec.BitRate,
		resampler.outgoingCodec.SampleRate,
		resampler.outgoingCodec.BitRate,
	)

	resamplers, contains := p.items[key]
	if !contains {
		p.items[key] = []*Resampler{resampler}

		return
	}

	resamplers = append(resamplers, resampler)

	p.items[key] = resamplers
}

func hash(
	inSampleRate int,
	inBitRate int,
	outSampleRate int,
	outBitRate int,
) int64 {

	return int64(inSampleRate&0x3FFFF)<<30 |
		int64(inBitRate&0x3F)<<24 |
		int64(outSampleRate&0x3FFFF)<<6 |
		int64(outBitRate&0x3F)

}
