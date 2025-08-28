package audiocodec

import (
	"fmt"
	"strings"
	"time"
)

var Pcm8kHz16bCodec = &Codec{
	Name:       Pcm,
	SampleRate: 8_000,
	BitRate:    16,
}

var Pcm16kHz16bCodec = &Codec{
	Name:       Pcm,
	SampleRate: 16_000,
	BitRate:    16,
}

var Pcm24kHz16bCodec = &Codec{
	Name:       Pcm,
	SampleRate: 24_000,
	BitRate:    16,
}

var Pcm44kHz32bCodec = &Codec{
	Name:       Pcm,
	SampleRate: 44_100,
	BitRate:    32,
}

var PcmA8kHz8bCodec = &Codec{
	Name:       PcmA,
	SampleRate: 8_000,
	BitRate:    8,
}

var PcmU8kHz8bCodec = &Codec{
	Name:       PcmU,
	SampleRate: 8_000,
	BitRate:    8,
}

type Codec struct {
	Name       Name `json:"name"`
	SampleRate int  `json:"sampleRate"`
	BitRate    int  `json:"bitRate"`
}

func (c *Codec) SampleSize() int {
	return c.BitRate / 8
}

func (c *Codec) Size(duration time.Duration) int {
	return c.SampleCountByDuration(duration) * c.SampleSize()
}

func (c *Codec) SizeBySampleCount(sampleCount int) int {
	return sampleCount * c.SampleSize()
}

func (c *Codec) Duration(size int) time.Duration {
	return time.Duration(c.SampleCountBySize(size)/(c.SampleRate/1000)) * time.Millisecond
}

func (c *Codec) SampleCountBySize(size int) int {
	return size / c.SampleSize()
}

func (c *Codec) SampleCountByDuration(duration time.Duration) int {
	return c.SampleRate * int(duration.Milliseconds()) / 1000
}

func (c *Codec) IsEqual(c2 *Codec) bool {
	return c.Name == c2.Name && c.SampleRate == c2.SampleRate && c.BitRate == c2.BitRate
}

func (c *Codec) IsPcm() bool {
	return c.Name.IsPcm()
}

func (c *Codec) Preset() Preset {
	return Preset(fmt.Sprintf("%s_%d_%d", c.Name, c.SampleRate, c.BitRate))
}

type Name string

const (
	Pcm  Name = "PCM"
	PcmA Name = "PCMA"
	PcmU Name = "PCMU"
)

func MustParseName(s string) Name {
	switch strings.ToUpper(s) {
	case "PCM":
		return Pcm
	case "PCMA":
		return PcmA
	case "PCMU":
		return PcmU
	}

	panic(fmt.Errorf("constant \"%s\" does not exist", s))
}

func (n Name) String() string {
	return string(n)
}

func (n Name) IsPcm() bool {
	return n == Pcm
}
