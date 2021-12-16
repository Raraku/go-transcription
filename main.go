package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	speech "cloud.google.com/go/speech/apiv1"
	"github.com/gordonklaus/portaudio"
	"google.golang.org/api/option"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

const samepleRate = 44100
const seconds = 1

func float32ToByte(f []float32) []byte {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, f)
	if err != nil {
		fmt.Println("binary.Write failed:", err)
	}
	return buf.Bytes()
}

func main() {
	portaudio.Initialize()
	defer portaudio.Terminate()
	ctx := context.Background()
	// numDevices, err := portaudio.Devices()
	// for _, dev := range numDevices {
	// 	fmt.Println(dev.Name)
	// }

	client, err := speech.NewClient(ctx, option.WithCredentialsFile("./google_creds.json"))
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Println(client)
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// Send the initial configuration message.
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: 16000,
					LanguageCode:    "en-US",
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}
	buf := make([]float32, 1024)
	portStream, err := portaudio.OpenDefaultStream(1, 0, samepleRate, len(buf), func(in []float32) {
		for i := range buf {
			buf[i] = in[i]
		}
	})
	fmt.Println(err)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	done := make(chan struct{})
	defer close(done)
	portStream.Start()
	defer portStream.Close()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	// Pipe stdin to the API.
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Cannot stream results: %v", err)
			}
			if err := resp.Error; err != nil {
				// Workaround while the API doesn't give a more informative error.
				if err.Code == 3 || err.Code == 11 {
					log.Print("WARNING: Speech recognition request exceeded limit of 60 seconds.")
				}
				log.Fatalf("Could not recognize: %v", err)
			}
			for _, result := range resp.Results {
				fmt.Printf("Result: %+v\n", result)
			}
		}
	}()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// n, err := os.Stdin.Read(buf)

			fmt.Println("storp")
			// portStream.Read()
			// portStream.Write()
			// fmt.Println(&buf)
			// if n > 0 {
			if err := stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
					AudioContent: float32ToByte(buf),
				},
			}); err != nil {
				log.Printf("Could not send audio: %v", err)
			}
			buf = make([]float32, 1024)

			if err == io.EOF {
				// Nothing else to pipe, close the stream.
				if err := stream.CloseSend(); err != nil {
					log.Fatalf("Could not close stream: %v", err)
				}
				return
			}
			if err != nil {
				log.Printf("Could not read from stdin: %v", err)
				continue
			}

		case <-interrupt:
			log.Println("interrupt")

			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}

}
