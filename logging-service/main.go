package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	utils "micro_basics/common"

	"github.com/hashicorp/consul/api"
	"github.com/hazelcast/hazelcast-go-client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

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

func StartNewServer(ctx context.Context, wg *sync.WaitGroup, hazelcastNode string, consulClient *api.Client) {
	wg.Add(1)

	config := hazelcast.Config{}
	config.Cluster.Name = "micro"
	config.Cluster.Network.SetAddresses(fmt.Sprintf("127.0.0.1:%s", hazelcastNode))

	s := NewLoggingService(config)

	port := utils.GetFreePort()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen on port %d: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	logging.RegisterLoggingServiceServer(grpcServer, s)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("logging.LoggingService", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	serviceID := fmt.Sprintf("logging-service:%d", port)
	reg := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    "logging-service",
		Address: "127.0.0.1",
		Port:    port,
		Check: &api.AgentServiceCheck{
			GRPC:     fmt.Sprintf("127.0.0.1:%d", port),
			Interval: "10s",
			Timeout:  "1s",
		},
	}
	consulClient.Agent().ServiceRegister(reg)

	go func() {
		defer wg.Done()
		log.Printf("gRPC server started on port %d\n", port)

		go func() {
			<-ctx.Done()
			log.Printf("shutting down gRPC server on port %d...\n", port)
			consulClient.Agent().ServiceDeregister(serviceID)
			grpcServer.GracefulStop()
		}()

		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server on port %d stopped: %v\n", port, err)
		}
	}()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	consulClient, _ := api.NewClient(api.DefaultConfig())

	kv := consulClient.KV()
	pair, _, _ := kv.Get("config", nil)

	var config map[string][]string
	json.Unmarshal(pair.Value, &config)
	hazelcastCluster := config["hazelcast-cluster"]

	for i, hazelcastNode := range hazelcastCluster {
		// Finish the second instance after 30 seconds
		if i == 1 {
			secondCtx, secondCancel := context.WithCancel(ctx)
			go StartNewServer(secondCtx, wg, hazelcastNode, consulClient)

			go func() {
				<-time.After(30 * time.Second)
				log.Println("Stopping second instance manually after 30 seconds")
				secondCancel()
			}()
		} else {
			go StartNewServer(ctx, wg, hazelcastNode, consulClient)
		}
	}

	<-sigs
	log.Println("Termination signal received")
	cancel()

	wg.Wait()
	log.Println("All services stopped cleanly")
}
