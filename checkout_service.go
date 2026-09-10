package main

import (
	"encoding/json"
	"net/http"
	"time"
)

type checkout struct {
	OrderID, CustomerID string
	TotalCents          int `json:"total_cents"`
}

func shouldRetry(status int) bool {
	return status == 0 || status >= 500 || status == http.StatusTooManyRequests
}

func checkoutHandler(q *queueClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in checkout
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.OrderID == "" {
			http.Error(w, "invalid checkout", http.StatusBadRequest)
			return
		}
		event := map[string]any{"event_id": "checkout-" + in.OrderID, "type": "order.created", "order_id": in.OrderID, "customer_id": in.CustomerID, "total_cents": in.TotalCents}
		if err := q.publish(event); err != nil {
			http.Error(w, "queue unavailable", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("queued"))
	}
}

func runWorker(q *queueClient, deliver func(map[string]any) int) {
	for {
		var messages []struct {
			MessageID string         `json:"message_id"`
			Payload   map[string]any `json:"payload"`
		}
		if err := q.consume(10, 30, &messages); err == nil {
			for _, m := range messages {
				if !shouldRetry(deliver(m.Payload)) {
					_ = q.ack(m.MessageID)
				}
			}
		}
		time.Sleep(time.Second)
	}
}
