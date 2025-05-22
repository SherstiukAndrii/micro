package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"micro_basics/common"
	"net"
	"net/http"
	"os"
	"strconv"
)

const (
	portStart = 3000
	portEnd   = 4000
)

type Config struct {
	HazelcastCluster []string `json:"hazelcast-cluster"`
	KafkaCluster     []string `json:"kafka-cluster"`
}

func findFreePort() (int, error) {
	for port := portStart; port <= portEnd; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free ports available")
}

type ConfigService struct {
	config          Config
	LoggingServices []string
	MessageServices []string
}

func NewConfigService() *ConfigService {
	var config Config
	file, _ := os.ReadFile("./config.json")
	json.Unmarshal(file, &config)

	fmt.Printf("Hazelcast cluster: %v\n", config.HazelcastCluster)
	fmt.Printf("Kafka cluster: %v\n", config.KafkaCluster)

	return &ConfigService{config, make([]string, 0), make([]string, 0)}
}

func (c *ConfigService) hazelcastClusterHandler(w http.ResponseWriter, r *http.Request) {
	hazelcastCluster := c.config.HazelcastCluster
	response, _ := json.Marshal(hazelcastCluster)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (c *ConfigService) kafkaClusterHandler(w http.ResponseWriter, r *http.Request) {
	kafkaCluster := c.config.KafkaCluster
	response, _ := json.Marshal(kafkaCluster)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (c *ConfigService) freeHandler(w http.ResponseWriter, r *http.Request) {
	port, err := findFreePort()
	if err == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strconv.Itoa(port)))
		return
	}
	http.Error(w, "No free ports available", http.StatusServiceUnavailable)
}

func (c *ConfigService) loggingServicesHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Logging services: %v\n", c.LoggingServices)
	loggingServices := c.LoggingServices
	response, _ := json.Marshal(loggingServices)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (c *ConfigService) addLoggingServiceHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	loggingService := string(body)
	r.Body.Close()

	for _, service := range c.LoggingServices {
		if service == loggingService {
			log.Printf("Logging service already exists: %s\n", loggingService)
			return
		}
	}

	c.LoggingServices = append(c.LoggingServices, loggingService)
	log.Printf("New logging service: %s\n", loggingService)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logging services added successfully"))
}

func (c *ConfigService) messageServicesHandler(w http.ResponseWriter, r *http.Request) {
	messageServices := c.MessageServices
	response, _ := json.Marshal(messageServices)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (c *ConfigService) addMessageServiceHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	messageService := string(body)
	r.Body.Close()

	for _, service := range c.MessageServices {
		if service == messageService {
			log.Printf("Message service already exists: %s\n", messageService)
			return
		}
	}

	c.MessageServices = append(c.MessageServices, messageService)
	log.Printf("New message service: %s\n", messageService)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Messages services added successfully"))
}

func main() {
	config := NewConfigService()

	http.HandleFunc("/free", config.freeHandler)
	http.HandleFunc("/hazelcast-cluster", config.hazelcastClusterHandler)
	http.HandleFunc("/kafka-cluster", config.kafkaClusterHandler)
	http.HandleFunc("/logging-services", config.loggingServicesHandler)
	http.HandleFunc("/add-logging-service", config.addLoggingServiceHandler)
	http.HandleFunc("/message-services", config.messageServicesHandler)
	http.HandleFunc("/add-message-service", config.addMessageServiceHandler)

	port, _ := findFreePort()
	os.WriteFile(common.ConfigServerPortPath, []byte(strconv.Itoa(port)), 0644)

	fmt.Printf("Logging-service started on %v...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
