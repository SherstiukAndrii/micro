package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/hazelcast/hazelcast-go-client"
	"google.golang.org/grpc"

	"micro_basics/common"
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

	return &LoggingService{hz: hz}
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

func StartNewServer(ctx context.Context, wg *sync.WaitGroup, hazelcastNode string, port string) {
	wg.Add(1)

	config := hazelcast.Config{}
	config.Cluster.Name = "micro"
	config.Cluster.Network.SetAddresses(fmt.Sprintf("127.0.0.1:%s", hazelcastNode))

	s := NewLoggingService(config)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	logging.RegisterLoggingServiceServer(grpcServer, s)

	go func() {
		defer wg.Done()
		log.Printf("gRPC server started on port %s\n", port)

		go func() {
			<-ctx.Done()
			log.Printf("shutting down gRPC server on port %s...\n", port)
			grpcServer.GracefulStop()
		}()

		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server on port %s stopped: %v\n", port, err)
		}
	}()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	configServerPort, _ := os.ReadFile(common.ConfigServerPortPath)
	configServerUrl := fmt.Sprintf("http://localhost:%s", string(configServerPort))

	resp, _ := http.Get(fmt.Sprintf("%s/hazelcast-cluster", configServerUrl))
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var hazelcastCluster []string
	json.Unmarshal(body, &hazelcastCluster)

	for _, hazelcastNode := range hazelcastCluster {
		resp, _ := http.Get(fmt.Sprintf("%s/free", configServerUrl))
		port, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		StartNewServer(ctx, wg, hazelcastNode, string(port))
		http.Post(fmt.Sprintf("%s/add-logging-service", configServerUrl), "text/plain", bytes.NewBufferString(string(port)))
	}

	<-sigs
	log.Println("Termination signal received")
	cancel()

	wg.Wait()
	log.Println("All services stopped cleanly")
}
