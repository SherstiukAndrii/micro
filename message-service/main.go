package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	utils "micro_basics/common"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/hashicorp/consul/api"
)

type MessageService struct {
	consumer *kafka.Consumer
	messages []string
}

func NewMessageService(config kafka.ConfigMap, node string) *MessageService {
	consumer, err := kafka.NewConsumer(&config)
	if err != nil {
		log.Fatalf("failed to create consumer: %v", err)
	}
	consumer.SubscribeTopics([]string{"test-topic2"}, nil)
	messageService := &MessageService{consumer, make([]string, 0)}

	go func() {
		for {
			msg, err := consumer.ReadMessage(-1)
			if err == nil {
				fmt.Printf("Message service %s received: %s\n", node, string(msg.Value))
				messageService.messages = append(messageService.messages, string(msg.Value))
			} else {
				fmt.Printf("Error: %v\n", err)
			}
		}
	}()
	return messageService
}

func (s *MessageService) messageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response, _ := json.Marshal(s.messages)
	w.Write(response)
}

func StartNewServer(ctx context.Context, wg *sync.WaitGroup, kafkaNode string, consulClient *api.Client) {
	wg.Add(1)

	config := kafka.ConfigMap{
		"bootstrap.servers": fmt.Sprintf("localhost:%s", kafkaNode),
		"group.id":          "test-group",
		"auto.offset.reset": "earliest",
	}

	s := NewMessageService(config, kafkaNode)

	mux := http.NewServeMux()
	mux.HandleFunc("/message", s.messageHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := utils.GetFreePort()
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	serviceID := fmt.Sprintf("message-service:%d", port)
	reg := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    "message-service",
		Address: "127.0.0.1",
		Port:    port,
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://127.0.0.1:%d/health", port),
			Interval: "10s",
			Timeout:  "1s",
		},
	}
	consulClient.Agent().ServiceRegister(reg)

	go func() {
		defer wg.Done()
		log.Printf("Starting server on port %d", port)

		go func() {
			<-ctx.Done()
			log.Printf("Shutting down server on port %d", port)
			consulClient.Agent().ServiceDeregister(serviceID)
			httpServer.Shutdown(context.Background())
		}()

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error on port %d: %v", port, err)
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
	kafkaCluster := config["kafka-cluster"]

	for i, kafkaNode := range kafkaCluster {
		if i > 1 {
			break
		}
		StartNewServer(ctx, wg, kafkaNode, consulClient)
	}

	<-sigs
	log.Println("Termination signal received")
	cancel()

	wg.Wait()
	log.Println("All services stopped cleanly")
}
