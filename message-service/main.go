package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"micro_basics/common"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
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

func StartNewServer(ctx context.Context, wg *sync.WaitGroup, kafkaNode string, port string) {
	wg.Add(1)

	config := kafka.ConfigMap{
		"bootstrap.servers": fmt.Sprintf("localhost:%s", kafkaNode),
		"group.id":          "test-group",
		"auto.offset.reset": "earliest",
	}

	s := NewMessageService(config, kafkaNode)

	mux := http.NewServeMux()
	mux.HandleFunc("/message", s.messageHandler)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	go func() {
		defer wg.Done()
		log.Printf("Starting server on port %s", port)

		go func() {
			<-ctx.Done()
			log.Printf("Shutting down server on port %s", port)
			httpServer.Shutdown(context.Background())
		}()

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error on port %s: %v", port, err)
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

	resp, _ := http.Get(fmt.Sprintf("%s/kafka-cluster", configServerUrl))
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var kafkaCluster []string
	json.Unmarshal(body, &kafkaCluster)

	for i, kafkaNode := range kafkaCluster {
		if i > 1 {
			break
		}
		resp, _ := http.Get(fmt.Sprintf("%s/free", configServerUrl))
		port, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		StartNewServer(ctx, wg, kafkaNode, string(port))
		http.Post(fmt.Sprintf("%s/add-message-service", configServerUrl), "text/plain", bytes.NewBufferString(string(port)))
		time.Sleep(5 * time.Second)
	}

	<-sigs
	log.Println("Termination signal received")
	cancel()

	wg.Wait()
	log.Println("All services stopped cleanly")
}
