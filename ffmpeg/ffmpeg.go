package ffmpeg

import (
	"fmt"
	"github.com/asticode/go-astiav"
	"sync"
	"time"
)

type VideoStream struct {
	Width     int
	Height    int
	Framerate float64
	Codec     string
}

type AudioStream struct {
	Channels   int
	SampleRate int
	Codec      string
}

type File struct {
	inputContext *astiav.FormatContext
	Metadata     map[string]string
	Duration     time.Duration
	videoStreams []VideoStream
	audioStreams []AudioStream
	accessLock   sync.Mutex
}

func Open(path string) (*File, error) {
	inputContext := astiav.AllocFormatContext()
	if err := inputContext.OpenInput(path, nil, nil); err != nil {
		return nil, fmt.Errorf("could not open input: %w", err)
	}

	f := &File{
		inputContext: inputContext,
		Metadata:     make(map[string]string),
	}

	// Retrieve the Metadata
	metadata := inputContext.Metadata()

	if metadata != nil {
		var previousMetadata *astiav.DictionaryEntry
		for {
			previousMetadata = metadata.Get("", previousMetadata, AV_DICT_IGNORE_SUFFIX)
			if previousMetadata == nil {
				break
			}
			f.Metadata[previousMetadata.Key()] = previousMetadata.Value()
			fmt.Printf("%s: %s\n", previousMetadata.Key(), previousMetadata.Value())
		}
	}

	// Retrieve the Duration
	duration := inputContext.Duration()
	f.Duration = time.Duration(duration) * time.Microsecond

	// Iterate over the streams to get codec information
	for i := 0; i < inputContext.NbStreams(); i++ {
		stream := inputContext.Streams()[i]
		codecParams := stream.CodecParameters()

		switch codecParams.CodecType() {
		case astiav.MediaTypeVideo:
			rational := stream.AvgFrameRate()
			vs := VideoStream{
				Width:     codecParams.Width(),
				Height:    codecParams.Height(),
				Framerate: float64(rational.Num()) / float64(rational.Den()),
				Codec:     astiav.FindDecoder(codecParams.CodecID()).Name(),
			}
			f.videoStreams = append(f.videoStreams, vs)
		case astiav.MediaTypeAudio:
			as := AudioStream{
				Channels:   codecParams.Channels(),
				SampleRate: codecParams.SampleRate(),
				Codec:      astiav.FindDecoder(codecParams.CodecID()).Name(),
			}
			f.audioStreams = append(f.audioStreams, as)
		}
	}

	return f, nil
}

func (f *File) GetKeyframes() int64 {
	f.accessLock.Lock()
	defer f.accessLock.Unlock()

	var keyframeIndexes []int64

}

func (f *File) Close() {
	f.inputContext.CloseInput()
}
