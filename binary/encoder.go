package binary

import "encoding/binary"

func Float32ToBytes16bit(sample float32, buf []byte) {
	var v uint16
	if sample < 0 {
		v = uint16(32_768 * sample)
	} else {
		v = uint16(32_767 * sample)
	}
	binary.LittleEndian.PutUint16(buf, v)
}

func Float32ToBytes32bit(sample float32, buf []byte) {
	var v uint32
	if sample < 0 {
		v = uint32(2_147_483_648 * sample)
	} else {
		v = uint32(2_147_483_647 * sample)
	}
	binary.LittleEndian.PutUint32(buf, v)
}
