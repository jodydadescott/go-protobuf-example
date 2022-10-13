package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pb "github.com/jodydadescott/go-protobuf-example/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	sync.Mutex
	xmap       map[string]*handler
	grpcServer *grpc.Server
}

func newServer() *server {
	return &server{
		xmap: make(map[string]*handler),
	}
}

func (t *server) start() error {

	if t.grpcServer != nil {
		return fmt.Errorf("Duplicate call to start()")
	}

	lis, err := net.Listen("unix", "/tmp/proto.sock")

	if err != nil {
		return err
	}

	t.grpcServer = grpc.NewServer()
	pb.RegisterStreamServiceServer(t.grpcServer, t)

	log.Println("start server")
	return t.grpcServer.Serve(lis)
}

func (t *server) shutdown() {

	log.Printf("(server) shutdown()")

	if t.grpcServer == nil {
		return
	}

	handlers := t.handlers()

	if len(handlers) > 0 {
		for _, v := range handlers {
			v.shutdown()
		}
	} else {
		log.Printf("No current handlers")
	}

	t.grpcServer.GracefulStop()
}

type handler struct {
	*server
	key    string
	closed chan struct{}
	wg     sync.WaitGroup
}

func (t *server) newHandler() *handler {
	t.Lock()
	defer t.Unlock()
	h := &handler{
		server: t,
		closed: make(chan struct{}),
		key:    random(),
	}
	t.xmap[h.key] = h
	log.Printf("New handler %s", h.key)
	return h
}

func (t *handler) shutdown() {
	log.Printf("(handler=%s) shutdown()", t.key)
	close(t.closed)
	t.wg.Wait()
}

func (t *handler) drop() {
	log.Printf("(handler=%s) drop()", t.key)
	t.Lock()
	defer t.Unlock()
	delete(t.xmap, t.key)
}

func (t *server) FetchResponse(in *pb.Request, srv pb.StreamService_FetchResponseServer) error {
	h := t.newHandler()
	return h.fetchResponse(in, srv)
}

func (t *server) handlers() []*handler {
	t.Lock()
	defer t.Unlock()
	var handlers []*handler
	for _, v := range t.xmap {
		handlers = append(handlers, v)
	}
	return handlers
}

func (t *handler) fetchResponse(in *pb.Request, srv pb.StreamService_FetchResponseServer) error {

	t.wg.Add(1)
	defer t.wg.Done()
	defer t.drop()

	log.Printf("fetch response for id : %d", in.Id)

	count := 0
	ticker := time.NewTicker(time.Duration(2) + time.Second)
	defer ticker.Stop()

	for {

		select {
		case <-t.closed:
			log.Printf("<-closed")
			ticker.Stop()
			return nil

		case <-ticker.C:
			log.Printf("<-ticker")
			//time sleep to simulate server process time
			resp := pb.Response{Result: fmt.Sprintf("Request #%d For Id:%d", count, in.Id)}
			err := srv.Send(&resp)

			if err != nil {
				if e, ok := status.FromError(err); ok {
					switch e.Code() {
					case codes.Unavailable:
						log.Printf("Listener went away")

						return nil
					}
				}

				log.Printf("Something is wrong %v", err)
				return err
			}

			log.Printf("finishing request number : %d", count)

			count++
		}

	}

}

func random() string {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		panic("could not generate nonce")
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

func main() {

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	server := newServer()

	go func() {
		log.Printf("Blocing on <-sig")
		<-sig
		log.Printf("NOT Blocing on <-sig")
		server.shutdown()
	}()

	log.Printf("Starting server")

	err := server.start()
	if err != nil {
		log.Fatalf("Something is wrong %v", err)
	}

}
