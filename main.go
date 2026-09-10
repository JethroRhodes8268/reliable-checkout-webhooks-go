package main

import (
	"log"
	"net/http"
)

func main() {
	q := newQueueClient()
	if err := q.createQueue(); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/checkout", checkoutHandler(q))
	go runWorker(q, func(map[string]any) int { return http.StatusOK })
	log.Println("checkout webhook service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
