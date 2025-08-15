package codec

import "errors"

var NotPcm = errors.New("allowed only PCM codec")
var IncomingAndOutgoingCodecsIsEquals = errors.New("incoming and outgoing codecs is equal")
