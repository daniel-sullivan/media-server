package main

import (
	"errors"
	"fmt"
	"log"
	"media_manager/ffmpeg"
	"time"

	"github.com/asticode/go-astiav"
)

func info() {
	// Initialize the library
	astiav.SetLogLevel(astiav.LogLevelInfo)

	// Open the media file
	filePath := "test/test.mkv"
	formatCtx := astiav.AllocFormatContext()
	if err := formatCtx.OpenInput(filePath, nil, nil); err != nil {
		log.Fatalf("could not open input: %v", err)
	}
	defer formatCtx.CloseInput()

	// Retrieve the metadata
	metadata := formatCtx.Metadata()

	if metadata != nil {
		var previousMetadata *astiav.DictionaryEntry
		for {
			previousMetadata = metadata.Get("", previousMetadata, ffmpeg.AV_DICT_IGNORE_SUFFIX)
			if previousMetadata == nil {
				break
			}
			fmt.Printf("%s: %s\n", previousMetadata.Key(), previousMetadata.Value())
		}
	} else {
		fmt.Println("No metadata found")
	}

	// Retrieve the duration
	duration := formatCtx.Duration()
	fmt.Printf("Duration: %v\n", time.Duration(duration)*time.Microsecond)

	// Iterate over the streams to get codec information
	for i := 0; i < formatCtx.NbStreams(); i++ {
		stream := formatCtx.Streams()[i]
		codecParams := stream.CodecParameters()
		codec := astiav.FindDecoder(codecParams.CodecID())
		fmt.Printf("Stream %d: Codec: %s\n", i, codec.Name())

		// Get resolution information
		if codecParams.CodecType() == astiav.MediaTypeVideo {
			rational := stream.AvgFrameRate()
			framerate := float64(rational.Num()) / float64(rational.Den())
			fmt.Printf("Stream %d: Framerate: %.2f fps\n", i, framerate)
			width := codecParams.Width()
			height := codecParams.Height()
			fmt.Printf("Stream %d: Resolution: %dx%d\n", i, width, height)
		}

		// Get codec details
		fmt.Printf("Stream %d: Codec Level: %d\n", i, codecParams.Level())
		fmt.Printf("Stream %d: Codec Profile: %d\n", i, codecParams.Profile())
	}

	var keyframeTimestamps []int64

	// Alloc packet
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	// Loop through packets
	for {
		// Read frame
		if err := formatCtx.ReadFrame(pkt); err != nil {
			if errors.Is(err, astiav.ErrEof) {
				break
			}
			log.Fatal(fmt.Errorf("main: reading frame failed: %w", err))
		}

		// Check if the packet is a keyframe
		if pkt.Flags().Has(astiav.PacketFlagKey) {
			keyframeTimestamps = append(keyframeTimestamps, pkt.Pts())
		}

		pkt.Unref()
	}

	// Print keyframe timestamps
	for _, ts := range keyframeTimestamps {
		fmt.Printf("Keyframe at PTS: %d\n", ts)
	}
}
