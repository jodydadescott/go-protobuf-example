package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/jodydadescott/go-protobuf-example/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {

	err := run()

	if err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {

	conn, err := grpc.Dial("passthrough:///unix:///tmp/proto.sock", grpc.WithInsecure())

	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())

	// create stream
	client := pb.NewStreamServiceClient(conn)
	in := &pb.Request{Id: 1}
	stream, err := client.FetchResponse(ctx, in)
	if err != nil {
		cancel()
		return err
	}

	streamdone := make(chan error)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Waiting on signal")
		<-sig
		log.Printf("Got signal")
		cancel()
	}()

	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				streamdone <- nil
				return
			}

			if err != nil {
				streamdone <- err
				return
			}

			if err != nil {
				if e, ok := status.FromError(err); ok {
					switch e.Code() {
					case codes.Canceled:
						log.Printf("It was cancelled")
						streamdone <- err
						return
					}
				}
				streamdone <- err
				return
			}

			log.Printf("Resp received: %s", resp.Result)
		}
	}()

	log.Printf("waiting on stream to end")
	<-streamdone //we will wait until all response is received
	log.Printf("stream ended")
	return err
}
