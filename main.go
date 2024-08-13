package main

import "C"
import (
	"errors"
	"fmt"
	"github.com/asticode/go-astiav"
	"log"
	"os"
	"strings"
)

var output = "test/test.mp4"

func main() {
	// Handle ffmpeg logs
	astiav.SetLogLevel(astiav.LogLevelDebug)
	astiav.SetLogCallback(func(c astiav.Classer, l astiav.LogLevel, fmt, msg string) {
		var cs string
		log.Printf("ffmpeg log: %s%s - level: %d\n", strings.TrimSpace(msg), cs, l)
	})

	info()

	// Alloc packet
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	// Alloc input format context
	inputFormatContext := astiav.AllocFormatContext()
	if inputFormatContext == nil {
		log.Fatal(errors.New("main: input format context is nil"))
	}
	defer inputFormatContext.Free()

	// Open input
	if err := inputFormatContext.OpenInput("test/test.mkv", nil, nil); err != nil {
		log.Fatal(fmt.Errorf("main: opening input failed: %w", err))
	}
	defer inputFormatContext.CloseInput()

	// Find stream info
	if err := inputFormatContext.FindStreamInfo(nil); err != nil {
		log.Fatal(fmt.Errorf("main: finding stream info failed: %w", err))
	}

	outputFormat := astiav.FindOutputFormat("mp4")

	if outputFormat == nil {
		log.Fatal(errors.New("main: could not find output format"))
	}

	// Alloc output format context
	outputFormatContext, err := astiav.AllocOutputFormatContext(outputFormat, "", "")
	if err != nil {
		log.Fatal(fmt.Errorf("main: allocating output format context failed: %w", err))
	}
	if outputFormatContext == nil {
		log.Fatal(errors.New("main: output format context is nil"))
	}
	defer outputFormatContext.Free()

	// Loop through streams
	inputStreams := make(map[int]*astiav.Stream)  // Indexed by input stream index
	outputStreams := make(map[int]*astiav.Stream) // Indexed by input stream index
	for _, is := range inputFormatContext.Streams() {
		// Only process audio or video
		if is.CodecParameters().MediaType() != astiav.MediaTypeAudio &&
			is.CodecParameters().MediaType() != astiav.MediaTypeVideo {
			continue
		}

		// Add input stream
		inputStreams[is.Index()] = is

		// Add stream to output format context
		os := outputFormatContext.NewStream(nil)
		if os == nil {
			log.Fatal(errors.New("main: output stream is nil"))
		}

		// Copy codec parameters
		if err = is.CodecParameters().Copy(os.CodecParameters()); err != nil {
			log.Fatal(fmt.Errorf("main: copying codec parameters failed: %w", err))
		}

		// Reset codec tag
		os.CodecParameters().SetCodecTag(0)

		// Add output stream
		outputStreams[is.Index()] = os
	}

	f, err := os.OpenFile("test/test.mp4", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(fmt.Errorf("main: opening output failed: %w", err))
	}
	defer f.Close()

	// If this is a file, we need to use an io context
	if !outputFormatContext.OutputFormat().Flags().Has(astiav.IOFormatFlagNofile) {
		ioContext, err := astiav.AllocIOContext(4092, true, f.Read, f.Seek, f.Write)
		if err != nil {
			log.Fatal(fmt.Errorf("main: opening io context failed: %w", err))
		}
		defer ioContext.Free() //nolint:errcheck

		// Update output format context
		outputFormatContext.SetPb(ioContext)
	}

	// Set the start and end time for the desired range (in seconds)
	startTime := 10
	endTime := 20

	// Seek to the start time
	if err := inputFormatContext.SeekFrame(-1, int64(startTime*astiav.TimeBase), astiav.NewSeekFlags(astiav.SeekFlagBackward)); err != nil {
		log.Fatal(fmt.Errorf("main: seeking to start time failed: %w", err))
	}

	// Write header
	if err = outputFormatContext.WriteHeader(nil); err != nil {
		log.Fatal(fmt.Errorf("main: writing header failed: %w", err))
	}

	var startPts int64
	for {
		// Read frame
		if err = inputFormatContext.ReadFrame(pkt); err != nil {
			if errors.Is(err, astiav.ErrEof) {
				break
			}
			log.Fatal(fmt.Errorf("main: reading frame failed: %w", err))
		}

		// Get input stream
		inputStream, ok := inputStreams[pkt.StreamIndex()]
		if !ok {
			pkt.Unref()
			continue
		}

		// Get output stream
		outputStream, ok := outputStreams[pkt.StreamIndex()]
		if !ok {
			pkt.Unref()
			continue
		}

		// Check if the packet is within the desired time range
		if pkt.Pts() != astiav.NoPtsValue && pkt.Pts()*int64(inputStream.TimeBase().Num()) > int64(endTime*inputStream.TimeBase().Den()) {
			break
		}

		if startPts == 0 {
			startPts = pkt.Pts()
		}

		pkt.SetDts(pkt.Dts() - startPts)
		pkt.SetPts(pkt.Pts() - startPts)

		// Update packet
		pkt.SetStreamIndex(outputStream.Index())
		pkt.RescaleTs(inputStream.TimeBase(), outputStream.TimeBase())
		pkt.SetPos(-1)

		// Write frame
		if err = outputFormatContext.WriteInterleavedFrame(pkt); err != nil {
			log.Fatal(fmt.Errorf("main: writing interleaved frame failed: %w", err))
		}
	}

	// Write trailer
	if err = outputFormatContext.WriteTrailer(); err != nil {
		log.Fatal(fmt.Errorf("main: writing trailer failed: %w", err))
	}

	// Success
	log.Println("success")
}
