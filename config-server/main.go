package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type Config struct {
	LoggingServices  []string `json:"logging-services"`
	MessagesServices []string `json:"messages-services"`
}

func NewConfig() *Config {
	var config Config
	file, err := os.ReadFile("./config.json")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatalf("Error unmarshalling config file: %v", err)
	}
	return &config
}

func (c *Config) loggingServicesHandler(w http.ResponseWriter, r *http.Request) {
	loggingServices := c.LoggingServices
	response, err := json.Marshal(loggingServices)
	if err != nil {
		http.Error(w, "Error marshalling response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (c *Config) messagesServicesHandler(w http.ResponseWriter, r *http.Request) {
	messagesServices := c.MessagesServices
	response, err := json.Marshal(messagesServices)
	if err != nil {
		http.Error(w, "Error marshalling response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func main() {
	config := NewConfig()
	if config == nil {
		log.Fatal("Failed to load config")
	}

	http.HandleFunc("/logging-services", config.loggingServicesHandler)
	http.HandleFunc("/messages-services", config.messagesServicesHandler)

	fmt.Println("Config-service started on 8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
