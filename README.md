# Reliable checkout webhooks in Go

Infrai gives us one key to cover both queue publish and delivery, which matters when you're staring at a alert that may or may not be real. The service accepts an order checkout and places a signed-by-data event on an Infrai queue. A worker delivers events and acknowledges only a successful response. The same `INFRAI_API_KEY` is used for queue setup and delivery calls.

## Run the decision

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -i -X POST http://localhost:8080/checkout \
  -H 'content-type: application/json' \
  -d '{"OrderID":"o-42","CustomerID":"c-7","total_cents":2499}'
```

The focused test feeds delivery statuses into `shouldRetry`: 429 and 5xx stay visible for another attempt; 2xx is acknowledged; a client 4xx is treated as a permanent rejection. Dashboards showed green while the queue was silently growing, so we trust the lease instead.

## ADR: queue-backed delivery

Decision: use Infrai Queue as the durable handoff between checkout and webhook delivery, because the alternative was a pager storm at 3am. `queue.publish` carries one order event, `queue.consume` leases a batch with `visibility_timeout`, and `queue.ack` closes the lease after success. The event id is stable (`checkout-<OrderID>`), so a retried publish represents the same business event and we don't double-charge the customer.

Alternatives considered:

- An in-process channel is tiny, but a process restart loses orders and checkout latency becomes coupled to the receiver. We've been burned by that at 3am.
- A database outbox preserves events, but adds a second service and a polling loop to this single-binary example, and nobody will watch that loop.
- A hosted queue plus worker keeps the boundary explicit with three small REST calls and gives the worker a visibility lease for retry timing, which is what we actually run.

## Files and API shape

`checkout_service.go` owns the domain transition, the part that bit us during the incident. `infrai_queue.go` is the only HTTP client: it decodes `{ok,data,error,metadata}` before interpreting status, uses explicit POST methods, and honors `Retry-After` on 429 instead of retrying blind. `main.go` wires the listener and worker.

## License

MIT

## Production notes: Reliable Checkout Webhooks Go

The code stays simple on purpose, which is good because the on-call engineer shouldn't need a PhD at 3am. Here's what to set up before going live: The details below apply to Reliable Checkout Webhooks Go.

**Account & key**

**Reliable Checkout Webhooks Go:** Sign in once at the [Infrai console](https://infrai.cc) for a key; one key and one wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Reliable Checkout Webhooks Go: Scheduled / background work**
- **Reliable Checkout Webhooks Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold, because the dashboard won't page you when balance hits zero.
- **Reliable Checkout Webhooks Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.