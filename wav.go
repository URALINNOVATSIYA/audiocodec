package audiocodec

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

type Wav struct {
	headers  []byte
	data     []byte
	codec    *Codec
	read     int
	editable bool
}

func NewWav(codec *Codec) *Wav {
	return &Wav{
		codec:    codec,
		editable: true,
	}
}

func NewWavFromBytes(b []byte) (*Wav, error) {
	if len(b) < 12 || !(b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F') || !(b[8] == 'W' && b[9] == 'A' && b[10] == 'V' && b[11] == 'E') {
		return nil, InvalidWAV
	}

	var (
		codecFound bool
		dataFound  bool
		dataStart  int
		dataSize   uint32
		c          Codec
	)

	i := 12
	n := len(b)
	for {
		if i+8 > n {
			break
		}
		chunkID0, chunkID1, chunkID2, chunkID3 := b[i], b[i+1], b[i+2], b[i+3]
		chunkSize := binary.LittleEndian.Uint32(b[i+4 : i+8])
		payloadStart := i + 8
		payloadEnd := payloadStart + int(chunkSize)
		if payloadEnd > n {
			return nil, TruncatedWAV
		}

		if chunkID0 == 'f' && chunkID1 == 'm' && chunkID2 == 't' && chunkID3 == ' ' {
			if chunkSize < 16 {
				return nil, fmt.Errorf("invalid fmt chunk: size=%d", chunkSize)
			}
			audioFormat := binary.LittleEndian.Uint16(b[payloadStart+0 : payloadStart+2])
			numChannels := binary.LittleEndian.Uint16(b[payloadStart+2 : payloadStart+4])
			sampleRate := binary.LittleEndian.Uint32(b[payloadStart+4 : payloadStart+8])
			bitsPerSample := binary.LittleEndian.Uint16(b[payloadStart+14 : payloadStart+16])

			switch audioFormat {
			case 1:
				c.Name = Pcm
			case 6:
				c.Name = PcmA
			case 7:
				c.Name = PcmU
			default:
				return nil, fmt.Errorf("%w: format tag=%d", UnsupportedFormat, audioFormat)
			}
			if numChannels != 1 {
				return nil, StereoNotSupported
			}
			c.SampleRate = int(sampleRate)
			c.BitRate = int(bitsPerSample)
			codecFound = true
		} else if chunkID0 == 'd' && chunkID1 == 'a' && chunkID2 == 't' && chunkID3 == 'a' {
			dataFound = true
			dataStart = payloadStart
			dataSize = chunkSize
		}

		step := int(chunkSize)
		if (chunkSize & 1) == 1 {
			step++
		}
		i = payloadStart + step
	}

	if !codecFound {
		return nil, fmt.Errorf("fmt chunk not found")
	}
	if !dataFound {
		return nil, fmt.Errorf("data chunk not found")
	}
	if int(dataSize) < 0 || dataStart > n-int(dataSize) {
		return nil, TruncatedWAV
	}

	hdr := b[:dataStart:dataStart]
	d := b[dataStart : dataStart+int(dataSize) : dataStart+int(dataSize)]

	return &Wav{
		headers:  hdr,
		data:     d,
		codec:    &c,
		read:     0,
		editable: false,
	}, nil
}

func (w *Wav) DataSize() int {
	return len(w.data)
}

func (w *Wav) Write(data []byte) (int, error) {
	if !w.editable {
		return 0, WavFileIsNotEditable
	}

	w.data = append(w.data, data...)
	return len(data), nil
}

func (w *Wav) Read(p []byte) (n int, err error) {
	if w.editable {
		w.editable = false
		w.prepareHeaders()
	}

	if len(p) == 0 {
		return 0, nil
	}

	if w.read >= len(w.headers)+len(w.data) {
		return 0, io.EOF
	}

	hSize := len(w.headers)
	if w.read < hSize {
		n = copy(p, w.headers[w.read:])
		w.read += n

		if n == len(p) {
			return n, nil
		}
	}

	dr := copy(p[n:], w.data[w.read-hSize:])
	w.read += dr
	n += dr

	return n, nil
}

func (w *Wav) prepareHeaders() {
	w.headers = make([]byte, w.headersSize())

	copy(w.headers[0:4], "RIFF")
	binary.LittleEndian.PutUint32(w.headers[4:8], uint32(w.riffSize())-8)
	copy(w.headers[8:12], "WAVE")

	// Chunk ID "fmt "
	copy(w.headers[12:16], "fmt ")
	binary.LittleEndian.PutUint32(w.headers[16:20], uint32(w.fmtChunkSize()-8))
	binary.LittleEndian.PutUint16(w.headers[20:22], uint16(w.compressionCode()))
	binary.LittleEndian.PutUint16(w.headers[22:24], 1)
	binary.LittleEndian.PutUint32(w.headers[24:28], uint32(w.codec.SampleRate))
	binary.LittleEndian.PutUint32(w.headers[28:32], uint32(w.codec.Size(time.Second)))
	binary.LittleEndian.PutUint16(w.headers[32:34], uint16(w.codec.SampleSize()))
	binary.LittleEndian.PutUint16(w.headers[34:36], uint16(w.codec.BitRate))

	if w.codec.Name == Pcm {
		// Chunk ID "data"
		copy(w.headers[36:40], "data")
		binary.LittleEndian.PutUint32(w.headers[40:44], uint32(len(w.data)))
	} else {
		binary.LittleEndian.PutUint16(w.headers[36:38], uint16(w.extraFormatSize()))

		// Chunk ID "fact"
		copy(w.headers[38:42], "fact")
		binary.LittleEndian.PutUint32(w.headers[42:46], uint32(4)) // Chunk Data Size
		binary.LittleEndian.PutUint32(w.headers[46:50], uint32(w.codec.SampleCountBySize(len(w.data))))

		// Chunk ID "data"
		copy(w.headers[50:54], "data")
		binary.LittleEndian.PutUint32(w.headers[54:58], uint32(len(w.data)))
	}
}

func (w *Wav) headersSize() int {
	return w.riffSize() - len(w.data)
}

func (w *Wav) compressionCode() int {
	switch w.codec.Name {
	case Pcm:
		return 1
	case PcmA:
		return 6
	case PcmU:
		return 7
	}

	panic(fmt.Errorf("compression code not found for \"%s\" codec", w.codec.Name))
}

func (w *Wav) ExportTo(writer io.Writer) (size int, err error) {
	if w.editable {
		w.editable = false
		w.prepareHeaders()
	}

	var n int
	if n, err = writer.Write(w.headers); err != nil {
		return 0, err
	}
	size += n

	if n, err = writer.Write(w.data); err != nil {
		return 0, err
	}
	size += n

	return size, nil
}

// Смещение	Размер 	Описание 			Значение
// 0x00 	4 		Chunk ID 			"RIFF" (0x52494646)
// 0x04 	4 		Chunk Data Size		(file size) - 8
// 0x08 	4 		RIFF Type			"WAVE" (0x57415645)
// 0x10 	*		Wave chunks (секции WAV-файла)
func (w *Wav) riffSize() int {
	return 12 + w.waveChunksSize()
}

// Существует довольно много типов секций, заданных для файлов WAV, но нужны только две из них:
// - секция формата ("fmt ")
// - секция данных ("data")
func (w *Wav) waveChunksSize() int {
	return w.fmtChunkSize() + w.dataChunkSize()
}

// Смещение	Размер 	Описание 					Значение
// 0x00 	4		Chunk ID					"fmt " (0x666D7420)
// 0x04 	4		Chunk Data Size 			16 + extra format bytes
// 0x08 	2 		Compression code 			1 - 65535
// 0x0a 	2 		Number of channels 			1 - 65535
// 0x0c 	4 		Sample rate					1 - 0xFFFFFFFF
// 0x10 	4 		Average bytes per second	1 - 0xFFFFFFFF
// 0x14 	2 		Block align 				1 - 65535
// 0x16 	2 		Significant bits per sample	2 - 65535
// 0x18 	2 		Extra format bytes			0 - 65535
// 0x1a 	* 		Дополнительные данные формата (Extra format bytes)

// Дополнительные данные формата (Extra Format Bytes)
// Величина указывает, сколько далее идет дополнительных данных, описывающих формат.
// Она отсутствует, если код сжатия 1 (uncompressed PCM file), но может присутствовать и иметь любую другую величину для
// других типов сжатия, зависящую от количества необходимых для декодирования данных.
// Если величина не выравнена на слово (не делится нацело на 2), должен быть добавлен дополнительный байт в конец данных,
// но величина должна оставаться невыровненной.
func (w *Wav) fmtChunkSize() int {
	if w.codec.Name == Pcm {
		return 24
	}

	return 26 + w.extraFormatSize()
}

func (w *Wav) extraFormatSize() int {
	return w.factSize()
}

// Смещение	Размер	Описание 			Величина
// 0x00 	4 		Chunk ID 			"fact" (0x66616374)
// 0x04 	4 		Chunk Data Size		зависит от формата
// 0x08		*		Данные, зависящие от формата (Format Dependant Data)

// Данные, зависящие от формата (Format Dependant Data)
// В настоящий момент задано только одно поле для данных, зависящих от формата.
// Это единственное 4-байтное значение, которое указывает число выборок в секции данных аудиосигнала.
// Эта величина может использоваться вместе с количеством выборок в секунду (Samples Per Second value) указанном в
// секции формата - для вычисления продолжительности звучания сигнала в секундах.
// По мере появления новых форматов WAVE секция fact будет расширена с добавлением полей после поля числа выборок.
// Программы могут использовать размер секции fact для определения, какие поля представлены в секции.
func (w *Wav) factSize() int {
	return 12
}

// Смещение	Размер 	Описание
// 0x00 	4 		Chunk ID
// 0x04 	4 		Chunk Data Size
// 0x08 	* 		Chunk Data Bytes
func (w *Wav) dataChunkSize() int {
	return 8 + len(w.data)
}
