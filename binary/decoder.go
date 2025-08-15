package binary

func Bytes16bitToFloat32(sample []byte) float32 {
	_ = sample[1]
	v := int16(sample[0]) | int16(sample[1])<<8
	if v < 0 {
		return float32(v) / 32_768
	} else {
		return float32(v) / 32_767
	}
}

func Bytes32bitToFloat32(sample []byte) float32 {
	_ = sample[3]
	v := int32(sample[0]) | int32(sample[1])<<8 | int32(sample[2])<<16 | int32(sample[3])<<24
	if v < 0 {
		return float32(v) / 2_147_483_648
	} else {
		return float32(v) / 2_147_483_647
	}
}
