package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/at-wat/ebml-go/webm"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
)

//CreateWebRTCConnection function to create webrtc connection and return answer
func CreateWebRTCConnection(ingestionAddress, streamKey, offerStr string) (*webrtc.SessionDescription, error) {
	var (
		audioWriter, videoWriter       webm.BlockWriteCloser
		audioBuilder, videoBuilder     *samplebuilder.SampleBuilder
		audioTimestamp, videoTimestamp uint32
	)

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
				},
			},
		},
	}

	// Create a MediaEngine object to configure the supported codec
	m := webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// Only support VP8 and OPUS, this makes our WebM muxer code simpler
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	audioBuilder = samplebuilder.New(10, &codecs.OpusPacket{})
	videoBuilder = samplebuilder.New(10, &codecs.VP8Packet{})

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Allow us to receive 1 audio track, and 2 video tracks
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}

	go func() {
		peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
			fmt.Println("Tracks are being added my dear")

			// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
			// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
			go func() {
				ticker := time.NewTicker(time.Second * 3)
				for range ticker.C {
					rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
					if rtcpSendErr != nil {
						fmt.Println(rtcpSendErr)
					}
				}
			}()

			fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().Name)

			streamURL := fmt.Sprintf("%s/%s", ingestionAddress, streamKey)
			for {
				// Read RTP packets being sent to Pion
				rtp, readErr := track.ReadRTP()
				if readErr != nil {
					if readErr == io.EOF {
						return
					}
					panic(readErr)
				}
				switch track.Kind() {
				case webrtc.RTPCodecTypeAudio:
					pushOpus(rtp, audioBuilder, audioWriter, audioTimestamp)
				case webrtc.RTPCodecTypeVideo:
					pushVP8(streamURL, rtp, videoBuilder, audioWriter, videoWriter, videoTimestamp)
				}
			}
		})
		peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
			fmt.Println(candidate)
		})

		// Set the handler for ICE connection state
		// This will notify you when the peer has connected/disconnected
		peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
			fmt.Printf("Connection State has changed %s \n", connectionState.String())
		})

		select {}
	}()

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	err = json.Unmarshal([]byte(offerStr), &offer)
	if err != nil {
		return nil, err
	}

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		return nil, err
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}

	return &answer, nil
}

func startFFmpeg(streamURL string, audioWriter, videoWriter webm.BlockWriteCloser, width, height int) {
	// Create a ffmpeg process that consumes MKV via stdin, and broadcasts out to Twitch
	ffmpeg := exec.Command("ffmpeg", "-re", "-i", "pipe:0", "-c:v", "libx264", "-preset", "veryfast", "-maxrate", "3000k", "-bufsize", "6000k", "-pix_fmt", "yuv420p", "-g", "50", "-c:a", "aac", "-b:a", "160k", "-ac", "2", "-ar", "44100", "-f", "flv", streamURL) //nolint
	ffmpegIn, _ := ffmpeg.StdinPipe()
	ffmpegOut, _ := ffmpeg.StderrPipe()
	if err := ffmpeg.Start(); err != nil {
		panic(err)
	}

	go func() {
		scanner := bufio.NewScanner(ffmpegOut)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	ws, err := webm.NewSimpleBlockWriter(ffmpegIn,
		[]webm.TrackEntry{
			{
				Name:            "Audio",
				TrackNumber:     1,
				TrackUID:        12345,
				CodecID:         "A_OPUS",
				TrackType:       2,
				DefaultDuration: 20000000,
				Audio: &webm.Audio{
					SamplingFrequency: 48000.0,
					Channels:          2,
				},
			}, {
				Name:            "Video",
				TrackNumber:     2,
				TrackUID:        67890,
				CodecID:         "V_VP8",
				TrackType:       1,
				DefaultDuration: 33333333,
				Video: &webm.Video{
					PixelWidth:  uint64(width),
					PixelHeight: uint64(height),
				},
			},
		})
	if err != nil {
		panic(err)
	}

	fmt.Printf("WebM saver has started with video width=%d, height=%d\n", width, height)
	audioWriter = ws[0]
	videoWriter = ws[1]
}

// Parse Opus audio and Write to WebM
func pushOpus(rtpPacket *rtp.Packet, audioBuilder *samplebuilder.SampleBuilder, audioWriter webm.BlockWriteCloser, audioTimestamp uint32) {
	audioBuilder.Push(rtpPacket)

	for {
		sample := audioBuilder.Pop()
		if sample == nil {
			return
		}
		if audioWriter != nil {
			audioTimestamp += sample.Samples
			t := audioTimestamp / 48
			if _, err := audioWriter.Write(true, int64(t), rtpPacket.Payload); err != nil {
				panic(err)
			}
		}
	}
}

// Parse VP8 video and Write to WebM
func pushVP8(streamURL string, rtpPacket *rtp.Packet, videoBuilder *samplebuilder.SampleBuilder, audioWriter, videoWriter webm.BlockWriteCloser, videoTimestamp uint32) {
	videoBuilder.Push(rtpPacket)

	for {
		sample := videoBuilder.Pop()
		if sample == nil {
			return
		}
		// Read VP8 header.
		videoKeyframe := (sample.Data[0]&0x1 == 0)
		if videoKeyframe {
			// Keyframe has frame information.
			raw := uint(sample.Data[6]) | uint(sample.Data[7])<<8 | uint(sample.Data[8])<<16 | uint(sample.Data[9])<<24
			width := int(raw & 0x3FFF)
			height := int((raw >> 16) & 0x3FFF)

			if videoWriter == nil || audioWriter == nil {
				// Initialize WebM saver using received frame size.
				startFFmpeg(streamURL, audioWriter, videoWriter, width, height)
			}
		}
		if videoWriter != nil {
			videoTimestamp += sample.Samples
			t := videoTimestamp / 90
			if _, err := videoWriter.Write(videoKeyframe, int64(t), sample.Data); err != nil {
				panic(err)
			}
		}
	}
}
