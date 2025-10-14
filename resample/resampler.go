package resample

import (
	"sync"

	spool "gitlab.twin24.ai/twin/common-go/support/pool"
)

type (
	Resampler[P comparable] interface {
		Resample(in []byte) ([]byte, error)
		Flush() ([]byte, error)
		Free() error
		Reset() error
		Params() P
	}

	Constructor[R Resampler[P], P comparable] = func(P) (R, error)

	Pool[R Resampler[P], P comparable] struct {
		mu    *sync.RWMutex
		pools map[P]*spool.SyncPool[R]

		ctor Constructor[R, P]
	}
)

func NewPool[R Resampler[P], P comparable](
	ctor Constructor[R, P],
) *Pool[R, P] {

	return &Pool[R, P]{
		pools: make(map[P]*spool.SyncPool[R]),
		mu:    &sync.RWMutex{},
		ctor:  ctor,
	}
}

func (p *Pool[R, P]) Get(
	params P,
) (R, error) {

	pool := p.getOrCreatePool(params)

	if pool != nil {
		v, err := pool.Get()

		if err == nil {
			return v, nil
		}
	}

	resampler, err := p.ctor(params)
	if err != nil {
		var zero R

		return zero, err
	}

	return resampler, nil
}

func (p *Pool[R, P]) Put(resampler R) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := resampler.Reset(); err != nil {
		return
	}

	params := resampler.Params()

	pool, contains := p.pools[params]

	if !contains {
		ctor := func() (R, error) {
			return p.ctor(params)
		}

		pool = spool.New[R](ctor)

		p.pools[params] = pool
	}

	pool.Put(resampler)
}

func (p *Pool[R, P]) getOrCreatePool(
	params P,
) *spool.SyncPool[R] {

	p.mu.Lock()
	defer p.mu.Unlock()

	if pool, contains := p.pools[params]; contains {
		return pool
	}

	ctor := func() (R, error) {
		return p.ctor(params)
	}

	pool := spool.New[R](ctor)

	p.pools[params] = pool

	return pool
}
