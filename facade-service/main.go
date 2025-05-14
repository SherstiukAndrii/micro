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

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"micro_basics/logging"
)

type FacadeService struct {
	loggingServices []logging.LoggingServiceClient
	messageServices []string
}

func NewFacadeService() (*FacadeService, error) {
	resp, err := http.Get("http://localhost:8081/logging-services")
	if err != nil {
		return nil, fmt.Errorf("failed to get logging services: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var loggingServices []string
	err = json.Unmarshal(body, &loggingServices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal logging services: %w", err)
	}

	logClients := make([]logging.LoggingServiceClient, len(loggingServices))
	for i, service := range loggingServices {
		conn, err := grpc.Dial(service, grpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("failed to connect to logging service %s: %w", service, err)
		}
		logClients[i] = logging.NewLoggingServiceClient(conn)
	}

	resp, err = http.Get("http://localhost:8081/messages-services")
	if err != nil {
		return nil, fmt.Errorf("failed to get messages services: %w", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var messageServices []string
	err = json.Unmarshal(body, &messageServices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal logging services: %w", err)
	}

	return &FacadeService{loggingServices: logClients, messageServices: messageServices}, nil
}

func (fs *FacadeService) getLoggingService() logging.LoggingServiceClient {
	return fs.loggingServices[rand.Intn(len(fs.loggingServices))]
}

func (fs *FacadeService) getMessageService() string {
	return fs.messageServices[rand.Intn(len(fs.messageServices))]
}

func (fs *FacadeService) getHandler(w http.ResponseWriter, r *http.Request) {
	logClient := fs.getLoggingService()

	res, _ := logClient.GetMessages(context.Background(), &logging.GetMessagesRequest{})

	message := fs.getMessageService()
	addr := fmt.Sprintf("http://%s/message", message)
	resp, _ := http.Get(addr)
	messageServiceText := ""
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	messageServiceText = string(body)

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

	fmt.Fprintf(w, "New message: UUID=%s\n", id)
}

func sendMessageWithRetry(client logging.LoggingServiceClient, uuid string, msg string, maxRetries int, retryDelay time.Duration) bool {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		resp, err := client.SaveMessage(ctx, &logging.SaveMessageRequest{Uuid: uuid, Msg: msg})
		if err == nil {
			if resp.Success {
				log.Printf("[attempt %d] Success: uuid=%s Client=%v", attempt, uuid, client)
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

var facade FacadeService

func main() {
	facadeService, err := NewFacadeService()
	if err != nil {
		log.Fatalf("Failed to create FacadeService: %v", err)
	}

	http.HandleFunc("/get", facadeService.getHandler)
	http.HandleFunc("/post", facadeService.postHandler)

	fmt.Println("Facade-service started on 8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
