# Codec

## Ресемплирование

### libsamplerate (Secret Rabbit Code)

Оф. сайт - http://www.mega-nerd.com/SRC/index.html

Установка для разработки:

```shell
sudo apt install libsamplerate0-dev libsamplerate0
```

Установка для использования скомпилированного приложения:

```shell
sudo apt install libsamplerate0
```

### SoX Resampler

SoX предоставляет три метода преобразования частоты дискретизации: линейный интерполятор, многофазный повторный
дискретизатор и метод имитационного аналогового фильтра Джулиуса О. Смита.

Оф. сайт - https://sourceforge.net/projects/soxr/

Установка для разработки:

```shell
sudo apt install libsoxr-dev libsoxr0
```

Установка для использования скомпилированного приложения:

```shell
sudo apt install libsoxr0
```

### libswresample

Является частью проекта FFmpeg и предназначена для обработки аудиоданных

Оф. сайт - https://ffmpeg.org/libswresample.html

Установка для разработки:

```shell
sudo apt install libswresample-dev libavutil-dev
```

Установка для использования скомпилированного приложения:

```shell
sudo apt install ffmpeg
```

### Бенчмарк

CPU: Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz\
Длительность аудио: 11.338 сек

| Бенчмарк                                                        | Время, ns/op | Выделено, B/op | Выделений, allocs/op | Задержка, мс |
|-----------------------------------------------------------------|:------------:|:--------------:|:--------------------:|:------------:|
| BenchmarkLibSampleRate/LibSampleRate-SincBestQuality-22to8-16   |  232436149   |       0        |          0           |      0       |
| BenchmarkLibSampleRate/LibSampleRate-SincMediumQuality-22to8-16 |   49007565   |       0        |          0           |      0       | 
| BenchmarkLibSampleRate/LibSampleRate-SincFastest-22to8-16       |   22553258   |       0        |          0           |      0       |
| BenchmarkLibSampleRate/LibSampleRate-ZeroOrderHold-22to8-16     |   2283887    |       0        |          0           |      0       |
| BenchmarkLibSampleRate/LibSampleRate-Linear-22to8-16            |   2316688    |       0        |          0           |      0       |
| BenchmarkSoxR/SoxR-Quick-22to8-16                               |    690326    |       0        |          0           |      0       |
| BenchmarkSoxR/SoxR-LowQuality-22to8-16                          |   1385553    |       0        |          0           |      40      |
| BenchmarkSoxR/SoxR-MediumQuality-22to8-16                       |   1462756    |       0        |          0           |      80      |
| BenchmarkSoxR/SoxR-HighQuality-22to8-16                         |   1556859    |       0        |          0           |      80      |
| BenchmarkSoxR/SoxR-VeryHighQuality-22to8-16                     |   2286487    |       0        |          0           |     160      |
| BenchmarkFFmpeg/FFmpeg-22to8-16                                 |   1327950    |       0        |          0           |      0       |


### Пример


```go
package main

import (
	_ "embed"
	"github.com/URALINNOVATSIYA/audiocodec"
	"github.com/URALINNOVATSIYA/audiocodec/resample/libswresample"
	"time"
)

//go:embed audio.wav
var incomingData []byte

func main() {
	var outgoingData []byte

	incomingDataLen := len(incomingData)
	// Вычисляем размер чанка входных данных
	incomingChunkSize := codec.Pcm16kHz16bCodec.Size(20 * time.Millisecond)

	resampler, err := libswresample.NewResampler(codec.Pcm16kHz16bCodec, codec.Pcm8kHz16bCodec, 20 * time.Millisecond)
	if err != nil {
		panic(err)
	}
	defer resampler.Free()
	
	var outgoingChunk []byte
	// Начинаем с 44, т.к. 44 байта в WAV файле с несжатым аудио это заголовки
	for pos := 44; pos < incomingDataLen; pos += incomingChunkSize {
		to := pos + incomingChunkSize
		if to > incomingDataLen {
			outgoingChunk, err = resampler.Resample(incomingData[pos:])
			if err != nil {
				panic(err)
			}
		} else {
			outgoingChunk, err = resampler.Resample(incomingData[pos:pos+incomingChunkSize])
			if err != nil {
				panic(err)
			}
		}

		if len(outgoingChunk) > 0 {
			outgoingData = append(outgoingData, outgoingChunk...)
		}
	}

	// Промываем внутренний буфер по окончании ресемплирования, т.к. во внутреннем буфере могут остаться необработанные
	// данные
	outgoingChunk, err = resampler.Flush()
	if err != nil {
		panic(err)
	}
	if len(outgoingChunk) > 0 {
		outgoingData = append(outgoingData, outgoingChunk...)
	}
}
```
