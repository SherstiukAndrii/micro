package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
    http.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "not implemented yet")
    })

    fmt.Println("Messages-service started on 8081")
    log.Fatal(http.ListenAndServe(":8081", nil))
}
