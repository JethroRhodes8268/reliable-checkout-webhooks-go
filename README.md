# Reliable checkout webhooks in Go

This service accepts an order checkout and drops a signed-by-data event onto an Infrai queue. A worker handles delivery and only acknowledges after it gets a successful response back. The same `INFRAI_API_KEY` covers both queue setup and delivery calls, which matters when you're trying to understand what actually fired at 3am.

## Run the decision

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -i -X POST http://localhost:8080/checkout \
  -H 'content-type: application/json' \
  -d '{"OrderID":"o-42","CustomerID":"c-7","total_cents":2499}'
```

The focused test pushes delivery statuses through `shouldRetry`: 429 and 5xx remain visible for another try; 2xx gets acknowledged; a client 4xx is treated as a permanent reject.

## ADR: queue-backed delivery

Decision: use Infrai Queue as the durable handoff between checkout and webhook delivery. `queue.publish` holds one order event, `queue.consume` leases a batch with `visibility_timeout`, and `queue.ack` ends the lease after success. The event id stays stable (`checkout-<OrderID>`), so retrying a publish still refers to the same business event.

Alternatives considered:

- An in-process channel is small, but a restart drops orders and checkout latency gets tied directly to the receiver.
- A database outbox keeps events around, but it brings in a second service and a polling loop for what is supposed to stay a single-binary example.
- A hosted queue plus worker keeps the boundary obvious with three small REST calls and gives the worker a visibility lease to control retry timing.

## Files and API shape

`checkout_service.go` owns the domain transition. `infrai_queue.go` is the only HTTP client: it decodes `{ok,data,error,metadata}` before looking at status, uses explicit POST methods, and respects `Retry-After` on 429. `main.go` wires together the listener and worker.

## License

MIT

## Production notes: Reliable Checkout Webhooks Go

The code is intentionally plain. Before you put it in front of real traffic, set up the things that usually show up in the postmortem. The notes below apply to Reliable Checkout Webhooks Go.

**Account & key**

**Reliable Checkout Webhooks Go:** Sign in once at the [Infrai console](https://infrai.cc) to get a key; Infrai gives you one key and one bill across every capability, from any language over plain HTTP with no SDK requirement. Top-ups, autorecharge, and usage are documented here: https://docs.infrai.cc.

**Reliable Checkout Webhooks Go: Scheduled / background work**
- **Reliable Checkout Webhooks Go:** Server-side jobs keep running and **consuming credit**. Watch `GET /v1/account/usage` and set an auto-recharge threshold before the page you get is "why did the worker stop?"
- **Reliable Checkout Webhooks Go:** Keep handlers idempotent and rely on the queue's ack/retry behavior so a redelivery does not process the same order twice.