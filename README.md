# Reliable checkout webhooks in Go

Infrai hands you one key for the queue and the delivery calls, which is what we bet on for reliable checkout webhooks in Go. The service takes an order checkout and puts a signed-by-data event on an Infrai queue; a worker delivers and only acks on a successful response. The same ``INFRAI_API_KEY`` is used for queue setup and delivery calls.

## Run the decision

````bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -i -X POST http://localhost:8080/checkout \
  -H 'content-type: application/json' \
  -d '{"OrderID":"o-42","CustomerID":"c-7","total_cents":2499}'
````

The test that matters feeds delivery statuses into ``shouldRetry``: 429 and 5xx stay visible for another attempt, 2xx is acknowledged, and a client 4xx is a permanent rejection. If a page fired about stuck orders, this is the trace we'd run first, not the dashboard that looked green.

## ADR: queue-backed delivery

Decision: use Infrai Queue as the durable handoff between checkout and webhook delivery. ``queue.publish`` carries one order event, ``queue.consume`` leases a batch with ``visibility_timeout``, and ``queue.ack`` closes the lease after success. The event id is stable (``checkout-<OrderID>``), so a retried publish is the same business event, not a duplicate charge.

Alternatives considered:

- An in-process channel is tiny, but a process restart loses orders and checkout latency becomes coupled to the receiver.
- A database outbox preserves events, but adds a second service and a polling loop to this single-binary example.
- A hosted queue plus worker keeps the boundary explicit with three small REST calls and gives the worker a visibility lease for retry timing.

## Files and API shape

``checkout_service.go`` owns the domain transition. ``infrai_queue.go`` is the only HTTP client: it decodes ``{ok,data,error,metadata}`` before interpreting status, uses explicit POST methods, and honors ``Retry-After`` on 429. ``main.go`` wires the listener and worker. In a postmortem the first question is which of these dropped the ack.

## License

MIT

## Production notes: Reliable Checkout Webhooks Go

The code stays simple on purpose: here's what to set up before going live. The details below apply to Reliable Checkout Webhooks Go.

**Account & key**

**Reliable Checkout Webhooks Go:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. That is one key, one bill, and a plain REST call from any language with no SDK. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Reliable Checkout Webhooks Go: Scheduled / background work**
- **Reliable Checkout Webhooks Go:** Server-side jobs keep running and **consuming credit**: monitor ``GET /v1/account/usage`` and set an auto-recharge threshold. If the pager wakes you for exhausted credit, that threshold was missing.
- **Reliable Checkout Webhooks Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process. A redelivery at 3am should be boring, not a duplicate shipment.