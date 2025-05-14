package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/hazelcast/hazelcast-go-client"
	"google.golang.org/grpc"

	"micro_basics/logging"
)

type LoggingService struct {
	logging.UnimplementedLoggingServiceServer
	mu sync.Mutex
	hz *hazelcast.Client
}

func NewLoggingService(config hazelcast.Config) *LoggingService {
	ctx := context.TODO()
	hz, err := hazelcast.StartNewClientWithConfig(ctx, config)
	if err != nil {
		panic(err)
	}

	return &LoggingService{
		hz: hz,
	}
}

func (s *LoggingService) SaveMessage(ctx context.Context, req *logging.SaveMessageRequest) (*logging.SaveMessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mapName := "messages"
	m, err := s.hz.GetMap(ctx, mapName)
	if err != nil {
		return nil, err
	}

	if _, err := m.Put(ctx, req.Uuid, req.Msg); err != nil {
		return nil, err
	}

	fmt.Printf("LoggingService::SaveMessage UUID=%s, msg=%s\n", req.Uuid, req.Msg)

	return &logging.SaveMessageResponse{Success: true}, nil
}

func (s *LoggingService) GetMessages(ctx context.Context, req *logging.GetMessagesRequest) (*logging.GetMessagesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mapName := "messages"
	m, err := s.hz.GetMap(ctx, mapName)
	if err != nil {
		return nil, err
	}

	entries, err := m.GetEntrySet(ctx)
	if err != nil {
		return nil, err
	}

	var msgs []string
	for _, entry := range entries {
		msgs = append(msgs, fmt.Sprintf("%s: %s", entry.Key, entry.Value))
	}

	return &logging.GetMessagesResponse{Messages: msgs}, nil
}

func StartNewServer(service string) {
	host, port, _ := net.SplitHostPort(service)
	p, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}

	// dirty hack to get gzcast port
	p -= 3000
	hzAddr := fmt.Sprintf("%s:%d", host, p)
	config := hazelcast.Config{}
	config.Cluster.Name = "micro"
	config.Cluster.Network.SetAddresses(hzAddr)
	s := NewLoggingService(config)

	grpcAddr := fmt.Sprintf(":%s", port)
	lis, _ := net.Listen("tcp", grpcAddr)
	grpcServer := grpc.NewServer()

	logging.RegisterLoggingServiceServer(grpcServer, s)
	fmt.Printf("Logging-service started on %v...", grpcAddr)
	if err := grpcServer.Serve(lis); err != nil {
		fmt.Printf("Failed to serve: %v\n", err)
	}
}

func main() {
	resp, err := http.Get("http://localhost:8081/logging-services")
	if err != nil {
		fmt.Printf("failed to get logging services: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("failed to read response body: %v", err)
	}

	var loggingServices []string
	err = json.Unmarshal(body, &loggingServices)
	if err != nil {
		fmt.Printf("failed to unmarshal logging services: %v", err)
	}

	for _, service := range loggingServices {
		go func(service string) {
			StartNewServer(service)
		}(service)
	}
	select {}
}
