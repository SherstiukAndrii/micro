package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	utils "micro_basics/common"
	"micro_basics/logging"
)

type FacadeService struct {
	consulClient  *api.Client
	kafkaProducer []*kafka.Producer
}

func NewFacadeService(consulClient *api.Client) (*FacadeService, error) {
	kv := consulClient.KV()
	pair, _, _ := kv.Get("config", nil)

	var config map[string][]string
	json.Unmarshal(pair.Value, &config)
	kafkaCluster := config["kafka-cluster"]

	kafkaProducers := make([]*kafka.Producer, len(kafkaCluster))
	for i, node := range kafkaCluster {
		producer, _ := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": fmt.Sprintf("localhost:%s", node)})
		kafkaProducers[i] = producer
	}

	return &FacadeService{consulClient, kafkaProducers}, nil
}

func (fs *FacadeService) getLoggingServiceClient() logging.LoggingServiceClient {
	loggingServiceEntries, _, _ := fs.consulClient.Health().Service("logging-service", "", true, nil)
	for _, entry := range loggingServiceEntries {
		fmt.Printf("Available logging service: ID %s, Addr %s, Port %d\n",
			entry.Service.ID, entry.Service.Address, entry.Service.Port)
	}
	entry := loggingServiceEntries[rand.Intn(len(loggingServiceEntries))]
	conn, _ := grpc.Dial(fmt.Sprintf("%s:%d", entry.Service.Address, entry.Service.Port), grpc.WithInsecure())
	return logging.NewLoggingServiceClient(conn)
}

func (fs *FacadeService) getMessageServiceAddr() string {
	messageServiceEntries, _, _ := fs.consulClient.Health().Service("message-service", "", true, nil)
	entry := messageServiceEntries[rand.Intn(len(messageServiceEntries))]
	return fmt.Sprintf("http://%s:%d", entry.Service.Address, entry.Service.Port)
}

func (fs *FacadeService) getHandler(w http.ResponseWriter, r *http.Request) {
	logClient := fs.getLoggingServiceClient()

	res, _ := logClient.GetMessages(context.Background(), &logging.GetMessagesRequest{})

	messageServiceAddr := fs.getMessageServiceAddr()
	resp, _ := http.Get(fmt.Sprintf("%s/message", messageServiceAddr))
	body, _ := io.ReadAll(resp.Body)
	var messageServiceText []string
	json.Unmarshal(body, &messageServiceText)
	resp.Body.Close()

	final := fmt.Sprintf(
		"Messages-service answer: %s\nLogging-service all messages: %v\n",
		messageServiceText,
		res.Messages,
	)

	w.Write([]byte(final))
}

func (fs *FacadeService) postHandler(w http.ResponseWriter, r *http.Request) {
	msg := r.FormValue("msg")
	if msg == "" {
		http.Error(w, "'msg' is missed", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	logClient := fs.getLoggingServiceClient()

	success := sendMessageWithRetry(logClient, id, msg, 3, time.Second)
	if !success {
		http.Error(w, "Failed to save message after several attempts", http.StatusServiceUnavailable)
		return
	}

	producer := fs.kafkaProducer[rand.Intn(len(fs.kafkaProducer))]
	topic := "micro-topic"
	err := producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte(msg),
	}, nil)
	if err != nil {
		http.Error(w, "Failed to send message to Kafka", http.StatusInternalServerError)
		return
	}
	// producer.Flush(15000)
	log.Printf("Message \"%s\" sent to Kafka %s\n", msg, producer)

	fmt.Fprintf(w, "New message \"%s\": UUID=%s\n", msg, id)
}

func sendMessageWithRetry(client logging.LoggingServiceClient, uuid string, msg string, maxRetries int, retryDelay time.Duration) bool {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		resp, err := client.SaveMessage(ctx, &logging.SaveMessageRequest{Uuid: uuid, Msg: msg})
		if err == nil {
			if resp.Success {
				log.Printf("[attempt %d] Success: uuid=%s", attempt, uuid)
			} else {
				log.Printf("[attempt %d] Duplicate: uuid=%s", attempt, uuid)
			}
			return true
		}

		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.DeadlineExceeded || st.Code() == codes.Unavailable) {
			log.Printf("[attempt %d] Error: %v. Retry %v\n", attempt, err, retryDelay)
			time.Sleep(retryDelay)
			continue
		} else {
			log.Printf("[attempt %d] Unexpected error: %v. Cancel\n", attempt, err)
			return false
		}
	}

	return false
}

func main() {
	port := utils.GetFreePort()
	serviceID := fmt.Sprintf("facade-service:%d", port)

	consulClient, _ := api.NewClient(api.DefaultConfig())
	reg := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    "facade-service",
		Port:    port,
		Address: "127.0.0.1",
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://127.0.0.1:%d/health", port),
			Interval: "10s",
			Timeout:  "1s",
		},
	}

	facadeService, _ := NewFacadeService(consulClient)

	http.HandleFunc("/get", facadeService.getHandler)
	http.HandleFunc("/post", facadeService.postHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	consulClient.Agent().ServiceRegister(reg)
	defer consulClient.Agent().ServiceDeregister(serviceID)

	fmt.Printf("Facade-service started on %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
