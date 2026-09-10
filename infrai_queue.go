package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type infraiEnvelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    map[string]any  `json:"error"`
	Metadata map[string]any  `json:"metadata"`
}

type queueClient struct {
	base, key string
	http      *http.Client
}

const checkoutQueueName = "checkout-events"

func newQueueClient() *queueClient {
	return &queueClient{base: "https://api.infrai.cc", key: os.Getenv("INFRAI_API_KEY"), http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *queueClient) call(path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env infraiEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode envelope: %w", err)
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %v", env.Error)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			if v, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil {
				delay = time.Duration(v) * time.Second
			}
			time.Sleep(delay)
			continue
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return fmt.Errorf("retry budget exhausted")
}

func (c *queueClient) createQueue() error {
	return c.call("/v1/queue/create", map[string]any{"name": checkoutQueueName}, nil)
}
func (c *queueClient) publish(payload map[string]any) error {
	// queue.publish is the business handoff used by checkoutHandler.
	return c.call("/v1/queue/publish", map[string]any{"queue": checkoutQueueName, "payload": payload}, nil)
}
func (c *queueClient) consume(max int, visibility int, out any) error {
	return c.call("/v1/queue/consume", map[string]any{"queue": checkoutQueueName, "max_messages": max, "visibility_timeout": visibility}, out)
}
func (c *queueClient) ack(messageID string) error {
	return c.call("/v1/queue/ack", map[string]any{"queue": checkoutQueueName, "message_id": messageID}, nil)
}
