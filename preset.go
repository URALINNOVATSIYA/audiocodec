package audiocodec

import (
	"fmt"
	"strings"
)

type Preset string

const (
	Pcm8kHz16bPreset  Preset = "PCM_8000_16"
	Pcm16kHz16bPreset Preset = "PCM_16000_16"
	Pcm24kHz16bPreset Preset = "PCM_24000_16"
	Pcm44kHz32bPreset Preset = "PCM_44100_32"
	PcmA8kHz8bPreset  Preset = "PCMA_8000_8"
	PcmU8kHz8bPreset  Preset = "PCMU_8000_8"
)

func MustParsePreset(s string) Preset {
	preset, err := ParsePreset(s)
	if err != nil {
		panic(err)
	}
	return preset
}

func ParsePreset(s string) (Preset, error) {
	switch strings.ToUpper(s) {
	case "PCM_8000_16":
		return Pcm8kHz16bPreset, nil
	case "PCM_16000_16":
		return Pcm16kHz16bPreset, nil
	case "PCM_24000_16":
		return Pcm24kHz16bPreset, nil
	case "PCM_44100_32":
		return Pcm44kHz32bPreset, nil
	case "PCMA_8000_8":
		return PcmA8kHz8bPreset, nil
	case "PCMU_8000_8":
		return PcmU8kHz8bPreset, nil
	default:
		return "", fmt.Errorf("preset \"%s\" does not exist", s)
	}
}

func (p Preset) ToCodec() *Codec {
	switch p {
	case Pcm8kHz16bPreset:
		return Pcm8kHz16bCodec
	case Pcm16kHz16bPreset:
		return Pcm16kHz16bCodec
	case Pcm24kHz16bPreset:
		return Pcm24kHz16bCodec
	case Pcm44kHz32bPreset:
		return Pcm44kHz32bCodec
	case PcmA8kHz8bPreset:
		return PcmA8kHz8bCodec
	case PcmU8kHz8bPreset:
		return PcmU8kHz8bCodec
	}

	panic(fmt.Errorf("constant \"%s\" does not exist", p))
}

func (p Preset) String() string {
	return string(p)
}
