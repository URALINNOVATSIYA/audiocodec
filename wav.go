package codec

import (
	"encoding/binary"
	"io"
	"time"
)

type Wav struct {
	data  []byte
	codec *Codec
}

func NewWav(codec *Codec) *Wav {
	return &Wav{
		codec: codec,
	}
}

func (w *Wav) DataSize() int {
	return len(w.data)
}

func (w *Wav) Write(data []byte) (int, error) {
	w.data = append(w.data, data...)
	return len(data), nil
}

func (w *Wav) ExportTo(writer io.Writer) (size int, err error) {
	size = w.riffSize()
	data := make([]byte, size)

	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(size)-8)
	copy(data[8:12], "WAVE")

	// Chunk ID "fmt "
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], uint32(w.fmtChunkSize()-8))
	binary.LittleEndian.PutUint16(data[20:22], uint16(w.codec.CompressionCode()))
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], uint32(w.codec.SampleRate))
	binary.LittleEndian.PutUint32(data[28:32], uint32(w.codec.Size(time.Second)))
	binary.LittleEndian.PutUint16(data[32:34], uint16(w.codec.SampleSize()))
	binary.LittleEndian.PutUint16(data[34:36], uint16(w.codec.BitRate))

	if w.codec.Name == Pcm {
		// Chunk ID "data"
		copy(data[36:40], "data")
		binary.LittleEndian.PutUint32(data[40:44], uint32(len(w.data)))
		copy(data[44:], w.data)
	} else {
		binary.LittleEndian.PutUint16(data[36:38], uint16(w.extraFormatSize()))

		// Chunk ID "fact"
		copy(data[38:42], "fact")
		binary.LittleEndian.PutUint32(data[42:46], uint32(4)) // Chunk Data Size
		binary.LittleEndian.PutUint32(data[46:50], uint32(w.codec.SampleCountBySize(len(w.data))))

		// Chunk ID "data"
		copy(data[50:54], "data")
		binary.LittleEndian.PutUint32(data[54:58], uint32(len(w.data)))
		copy(data[58:], w.data)
	}

	return writer.Write(data)
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
