package audiocodec

import "errors"

var (
	NotPcm                            = errors.New("allowed only PCM codec")
	IncomingAndOutgoingCodecsIsEquals = errors.New("incoming and outgoing codecs is equal")
	WavFileIsNotEditable              = errors.New("wav file is not editable")
	InvalidWav                        = errors.New("invalid WAV: missing RIFF/WAVE")
	TruncatedWav                      = errors.New("invalid WAV: truncated chunk")
	UnsupportedFormat                 = errors.New("unsupported WAV format")
	OnlyMonoSupported                 = errors.New("unsupported WAV: only mono supported by Codec")
)
