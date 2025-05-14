package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	resp, err := http.Get("http://localhost:8081/messages-services")
	if err != nil {
		fmt.Printf("failed to get logging services: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("failed to read response body: %v", err)
	}

	var addr []string
	err = json.Unmarshal(body, &addr)
	if err != nil {
		fmt.Printf("failed to unmarshal logging services: %v", err)
	}

	http.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "not implemented yet")
	})

	for _, addr := range addr {
		// go func(addr string) {
			fmt.Printf("Messages-service started on %s\n", addr)
			log.Fatal(http.ListenAndServe(addr, nil))
		// }(addr)
	}
}
