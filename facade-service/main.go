package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"micro_basics/logging"
)

func sendMessageWithRetry(client logging.LoggingServiceClient, uuid string, msg string, maxRetries int, retryDelay time.Duration) bool {
    for attempt := 1; attempt <= maxRetries; attempt++ {
        ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
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

		success := sendMessageWithRetry(logClient, id, msg, 3, time.Second)
        if !success {
            http.Error(w, "Failed to save message after several attempts", http.StatusServiceUnavailable)
            return
        }

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
