package main

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"

	"micro_basics/logging"
)

type server struct {
    logging.UnimplementedLoggingServiceServer
    mu       sync.Mutex
    messages map[string]string
}

func (s *server) SaveMessage(ctx context.Context, req *logging.SaveMessageRequest) (*logging.SaveMessageResponse, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.messages[req.Uuid] = req.Msg
    fmt.Printf("LoggingService::SaveMessage UUID=%s, msg=%s\n", req.Uuid, req.Msg)

    return &logging.SaveMessageResponse{Success: true}, nil
}

func (s *server) GetMessages(ctx context.Context, req *logging.GetMessagesRequest) (*logging.GetMessagesResponse, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    var msgs []string
    for _, msg := range s.messages {
        msgs = append(msgs, msg)
    }

    return &logging.GetMessagesResponse{Messages: msgs}, nil
}

func main() {
    lis, _ := net.Listen("tcp", ":8082")

    grpcServer := grpc.NewServer()

    s := &server{
        messages: make(map[string]string),
    }
    logging.RegisterLoggingServiceServer(grpcServer, s)
    fmt.Println("Logging-service started on 8082...")

    grpcServer.Serve(lis)
}
