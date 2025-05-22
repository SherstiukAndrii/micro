package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"micro_basics/common"
	"micro_basics/logging"
)

type FacadeService struct {
	loggingServices []logging.LoggingServiceClient
	configServerUrl string
	kafkaProducer   []*kafka.Producer
}

func NewFacadeService() (*FacadeService, error) {
	configServerPort, _ := os.ReadFile(common.ConfigServerPortPath)
	configServerUrl := fmt.Sprintf("http://localhost:%s", string(configServerPort))

	resp, _ := http.Get(fmt.Sprintf("%s/logging-services", configServerUrl))
	body, _ := io.ReadAll(resp.Body)

	var loggingServices []string
	json.Unmarshal(body, &loggingServices)
	fmt.Printf("Logging services: %v\n", loggingServices)
	resp.Body.Close()

	logClients := make([]logging.LoggingServiceClient, len(loggingServices))
	for i, service := range loggingServices {
		conn, err := grpc.Dial(fmt.Sprintf("localhost:%s", service), grpc.WithInsecure())
		if err != nil {
			log.Fatalf("Failed to connect to logging service: %v", err)
		}
		logClients[i] = logging.NewLoggingServiceClient(conn)
	}

	resp, _ = http.Get(fmt.Sprintf("%s/kafka-cluster", configServerUrl))
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	var kafkaCluster []string
	json.Unmarshal(body, &kafkaCluster)
	kafkaProducers := make([]*kafka.Producer, len(kafkaCluster))
	for i, node := range kafkaCluster {
		producer, _ := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": fmt.Sprintf("localhost:%s", node)})
		kafkaProducers[i] = producer
	}

	return &FacadeService{loggingServices: logClients, configServerUrl: configServerUrl, kafkaProducer: kafkaProducers}, nil
}

func (fs *FacadeService) getLoggingService() logging.LoggingServiceClient {
	return fs.loggingServices[rand.Intn(len(fs.loggingServices))]
}

func (fs *FacadeService) getMessageService() string {
	resp, _ := http.Get(fmt.Sprintf("%s/message-services", fs.configServerUrl))
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var messageServices []string
	json.Unmarshal(body, &messageServices)

	return messageServices[rand.Intn(len(messageServices))]
}

func (fs *FacadeService) getHandler(w http.ResponseWriter, r *http.Request) {
	logClient := fs.getLoggingService()

	res, _ := logClient.GetMessages(context.Background(), &logging.GetMessagesRequest{})

	message := fs.getMessageService()
	resp, _ := http.Get(fmt.Sprintf("http://localhost:%s/message", message))
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
	logClient := fs.getLoggingService()

	success := sendMessageWithRetry(logClient, id, msg, 3, time.Second)
	if !success {
		http.Error(w, "Failed to save message after several attempts", http.StatusServiceUnavailable)
		return
	}

	producer := fs.kafkaProducer[rand.Intn(len(fs.kafkaProducer))]
	topic := "test-topic2"
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
	facadeService, _ := NewFacadeService()

	http.HandleFunc("/get", facadeService.getHandler)
	http.HandleFunc("/post", facadeService.postHandler)

	resp, _ := http.Get(fmt.Sprintf("%s/free", facadeService.configServerUrl))
	port, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	fmt.Printf("Facade-service started on %v...\n", string(port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", string(port)), nil))
}
