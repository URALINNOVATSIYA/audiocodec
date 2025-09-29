package audiocodec

import "errors"

var (
	NotPcm                            = errors.New("allowed only PCM codec")
	IncomingAndOutgoingCodecsIsEquals = errors.New("incoming and outgoing codecs is equal")
	WavFileIsNotEditable              = errors.New("wav file is not editable")
	InvalidWAV                        = errors.New("invalid WAV: missing RIFF/WAVE")
	TruncatedWAV                      = errors.New("invalid WAV: truncated chunk")
	UnsupportedFormat                 = errors.New("unsupported WAV format")
	StereoNotSupported                = errors.New("unsupported WAV: only mono supported by Codec")
)
