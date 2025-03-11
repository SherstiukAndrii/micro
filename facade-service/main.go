package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"micro_basics/logging"
)

func main() {
    conn, _ := grpc.Dial("localhost:8082", grpc.WithInsecure())
    defer conn.Close()

    logClient := logging.NewLoggingServiceClient(conn)

    http.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
        msg := r.FormValue("msg")
        if msg == "" {
            http.Error(w, "'msg' is missed", http.StatusBadRequest)
            return
        }

        id := uuid.New().String()
        logClient.SaveMessage(context.Background(), &logging.SaveMessageRequest{
            Uuid: id,
            Msg:  msg,
        })
        fmt.Fprintf(w, "New message: UUID=%s\n", id)
    })

    http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
        res, _ := logClient.GetMessages(context.Background(), &logging.GetMessagesRequest{})

        resp, _ := http.Get("http://localhost:8081/message")
        messagesServiceText := ""
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		messagesServiceText = string(body)

        final := fmt.Sprintf(
            "Messages-service answer: %s\nLogging-service all messages: %v\n",
            messagesServiceText,
            res.Messages,
        )

        w.Write([]byte(final))
    })

    fmt.Println("Facade-service started on 8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
