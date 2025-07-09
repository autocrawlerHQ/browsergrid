# BrowserMux Hybrid Webhook System

### Software Design Document — v1.0

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [System Overview](#system-overview)
3. [Design Objectives](#design-objectives)
4. [Architecture Overview](#architecture-overview)
5. [Core Components](#core-components)
6. [Webhook Behaviour](#webhook-behaviour)
7. [Decision Engine](#decision-engine)
8. [Blocking‑Rule Schema](#blocking-rule-schema)
9. [Data Flow](#data-flow)
10. [API Design](#api-design)
11. [Performance Considerations](#performance-considerations)
12. [Security & Reliability](#security-and-reliability)
13. [Implementation Strategy](#implementation-strategy)
14. [Monitoring & Observability](#monitoring-and-observability)
15. [Future Considerations](#future-considerations)
16. [Appendix A – Sample Blocking Rules](#appendix-a)
17. [Appendix B – Priority Matrix](#appendix-b)
18. [Appendix C – Timeout & Retry Profiles](#appendix-c)
19. [Glossary](#glossary)
20. [References](#references)

---

## Executive Summary

BrowserMux is an in‑process proxy that intercepts Chrome DevTools Protocol (CDP) commands, allowing external webhooks to observe, modify, or halt browser automation in real time. A single webhook object, augmented by a declarative *blocking rule*, yields both asynchronous (fire‑and‑forget) and synchronous (blocking) behaviour—removing the complexity of distinct webhook “types.”

> **Key innovation:** Dynamic downgrading—when a blocking rule does **not** match, BrowserMux silently flips the hook to async, preserving near‑zero latency for benign traffic.

**Primary use‑cases**

* Auto‑captcha solving (watch passively, block critical clicks)
* Fraud & abuse prevention (pause checkout if heuristics trigger)
* Compliance gates (block navigation to disallowed domains)

---

## System Overview

### Current State

| Aspect    | Current                |
| --------- | ---------------------- |
| Trigger   | After‑event only       |
| Execution | Async fire‑and‑forget  |
| Mutation  | None – read‑only       |
| Coverage  | Notification / logging |

### Target State

| Enhancement             | Description                                    |
| ----------------------- | ---------------------------------------------- |
| Single webhook model    | `blocking: bool` + optional `blocking_rule`    |
| Pre‑action interception | Modify / reject CDP before browser sees it     |
| Priority orchestration  | Multiple hooks ordered, parallel same‑priority |
| Rule intelligence       | URL, method, param & DOM‑based matching        |
| Observability           | Fine‑grained metrics, tracing & dashboards     |

---

## Design Objectives

### Primary

1. **Performance:** 95 % of actions add ≤ 200 µs (P99).
2. **Control:** Deterministic interception of high‑risk actions.
3. **Intelligence:** Rule engine minimises false positives.
4. **Reliability:** Graceful degradation (timeouts, circuit‑breakers).

### Secondary

* First‑class developer ergonomics (declarative YAML/JSON).
* Sandbox / replay for debugging.
* Horizontal scaling & statelessness.

---

## Architecture Overview

```
┌────────────┐   1. CDP Cmd   ┌────────────────────┐   3. Route   ┌──────────────┐
│  Browser   ├───────────────>│    BrowserMux      ├─────────────>│  Executor    │
│  Client    │                │ (Decision Engine)  │              └──────────────┘
└────────────┘                └─────────┬──────────┘
                                        │
                                 2. Dispatch
                                        │
                                 ┌──────▼───────┐
                                 │ Webhook Svc  │
                                 └──────▲───────┘
                                        │
                                 5. Response
                                        │
                                 ┌──────┴───────┐
                                 │ Trace+Metric │
                                 └──────────────┘
                                        ▲
                                        │
                                 4. Trace+Metrics
```

*Step numbers match interaction points—see § Data Flow.*

---

## Core Components

| # | Component           | Responsibilities                                                         |
| - | ------------------- | ------------------------------------------------------------------------ |
| 1 | **Registry**        | CRUD for webhook docs; hot‑reload                                        |
| 2 | **Decision Engine** | Evaluate `blocking` flag + rule; supply route                            |
| 3 | **Executor**        | Async worker pool & sync workers with timeout, retries, circuit‑breakers |
| 4 | **Aggregator**      | Merge multi‑hook results (`block` > `modify` > `continue`)               |

---

## Webhook Behaviour

| Field           | Description                                                    |
| --------------- | -------------------------------------------------------------- |
| `timing`        | `before_event` (can block) / `after_event` (always async)      |
| `blocking`      | `true` = synchronous *if* rule matches; `false` = always async |
| `blocking_rule` | Declarative conditions & action spec                           |
| `timeout`       | Seconds for sync call (soft UX cap 5 s)                        |
| `max_retries`   | Exponential back‑off attempts                                  |

> **Downgrade logic:** `blocking: true` + *rule miss* ⇒ execute async.

---

## Decision Engine

### Evaluation Layers

1. **Static:** flag, priority, feature flags.
2. **Rule:** method / URL / params / DOM.
3. **Context:** historical outcomes, system load.

### Conflict‐Handling

Deterministic precedence → `block` > `modify` > `continue` (ties by priority).
Back‑pressure semaphore = 16 × priority tier; excess ⇒ HTTP 429.

### Circuit‑Breaker

Trip after 5 consecutive failures *or* 50 % failures in 30 s roll‑window; half‑open after 60 s.

---

## Blocking‑Rule Schema

```yaml
blocking_rule:
  id: captcha_basic
  enabled: true
  priority: 100          # higher first

  triggers:
    cdp_method: "Input.*"
    url: "*"
    params:
      type: "mousePressed"
    content:
      contains: ["recaptcha","hcaptcha"]

  action:
    mode: block          # block | allow | modify
    webhook_url: https://solver.example/solve
    timeout: 30
    retry_count: 2
```

*Full JSONSchema in repository /docs/schemas/blocking\_rule.json.*

---

## Data Flow

### Async Path

`CDP → Dispatch → Async Queue → Webhook → (ignore) → Browser`

### Blocking Path

`CDP → Decision Engine → Sync Webhook(s) → Aggregator → Browser`

### Mixed (“Intelligent”)

If rule miss → async; rule hit → blocking (automatic per webhook).

---

## Webhook Resolution & Client Integration

### 1. Lifecycle States

| #  | State               | Who Owns It | Description                                                              |
| -- | ------------------- | ----------- | ------------------------------------------------------------------------ |
| 1  | **DISPATCHED**      | BrowserMux  | HTTP request sent to webhook endpoint.                                   |
| 2  | **ACKED**           | Webhook     | Endpoint has accepted the request (2xx or 202).                          |
| 3  | **EVALUATED**       | Webhook     | JSON body with `action` arrives at BrowserMux.                           |
| 4a | **CONTINUE**        | Aggregator  | `action=continue` – original CDP command passes through unchanged.       |
| 4b | **MODIFY**          | Aggregator  | `action=modify` – parameters merged, then command sent.                  |
| 4c | **BLOCK**           | Aggregator  | `action=block` – command suppressed; client sees “blocked” error.        |
| 4d | **RETRY_PENDING**   | Executor    | `retry_after_ms` present – command queued for re-evaluation after delay. |
| 5  | **TIMEOUT / ERROR** | Executor    | Hook failed—fallback behaviour (`allow`, `block`, or `retry`) applied.   |

### 2. Webhook Response Contract (recap)

```jsonc
{
  "action": "continue | block | modify",
  "delay":   0,                  // optional ms pause before forwarding
  "retry_after_ms": 500,         // optional retry window (block until next attempt)
  "modify_data": { ... },        // only if action = modify
  "message": "human-readable status"
}
```

*If the service needs more time* (e.g., captcha still solving), return
`"action": "block", "retry_after_ms": 1000`.
BrowserMux holds the CDP command in an in-memory delay queue and re-calls the same webhook until it returns `continue` or the hard timeout is hit.

### 3. Aggregation Rules (multiple blocking hooks)

1. Collect all synchronous responses.
2. Sort by **precedence** (`block` > `modify` > `continue`).
3. If the top result is **block** *and* any response contains `retry_after_ms`, enter **RETRY_PENDING**; otherwise block outright.
4. If top result is **modify**, merge `modify_data` objects field-by-field (last-writer-wins when keys collide).
5. Emit a single resolution event with `resolution_id` for downstream auditing.

### 4. What the Browser-Automation Client Sees

| Scenario                 | CDP Result                                                   | Suggested Client Handling                                                |
| ------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------------------ |
| Continue (or async)      | Normal CDP reply                                             | None – acts as if BrowserMux is transparent.                             |
| Modify                   | CDP reply reflects new params (e.g., different URL)          | Client libraries should treat it as normal success.                      |
| Block (final)            | CDP error `BrowserMux.Blocked` with message field            | Caller decides: abort script, retry later, or surface UI.                |
| Retry-Pending            | No CDP reply yet (command is paused)                         | Automation run blocks until hook resolves or the 30 s hard limit is hit. |
| Timeout Fallback = allow | Command proceeds; “timeout-fallback” tag added to audit log. |                                                                          |
| Timeout Fallback = block | Same error path as explicit block.                           |                                                                          |

### 5. Idempotency & Deduplication

* Each webhook request carries `X-BMux-Request-ID` (UUID).
* Webhook services **must** treat the request idempotently—returning the same decision when the same ID is received again (e.g., after retry).
* BrowserMux caches the final decision for that ID for 10 minutes to prevent replay storms.

### 6. Client-Side Helper Library (optional)

For common languages (Python, Node, Go) we will ship a thin wrapper that:

* Converts `BrowserMux.Blocked` errors into exceptions with retry metadata.
* Exposes a `wait_for_unblock()` convenience that polls the command status endpoint `/api/commands/{id}` if the caller prefers asynchronous unwinding instead of blocking the CDP socket.
* Emits structured logs (`resolution_id`, latency_ms, rule_name) for local debugging.

### 7. Observability Hooks

* Every state transition (see §1) produces an internal event shipped to the `bmux.webhook.lifecycle` stream; this powers both the Prometheus counters and the per-command timeline in the tracing UI.
* A stuck command (still in **RETRY_PENDING** beyond 30 s) triggers alert `BMUX_RETRY_STALL`.

---

## API Design

### Create Webhook (excerpt)

```http
POST /api/webhooks
{
  "name": "Captcha Hook",
  "url":  "https://solver.example.com/webhook",
  "event_method": "Input.dispatchMouseEvent",
  "timing": "before_event",
  "blocking": true,
  "priority": 120,
  "blocking_rule": { … },
  "timeout": 30,
  "max_retries": 2,
  "enabled": true,
  "mTLS": true,
  "client_cert_secret": "captcha-cert"
}
```

### Blocking Response

```json
{ "action": "block", "retry_after_ms": 500, "message": "Captcha solving" }
```

---

## Performance Considerations

| Metric         | P50      | P99      | Notes           |
| -------------- | -------- | -------- | --------------- |
| Async dispatch | < 100 µs | < 500 µs | goroutine queue |
| Rule eval      | < 50 µs  | < 200 µs | cached trie     |
| Sync exec      | n/a      | ≤ 30 s   | soft cap 5 s UX |

Throughput targets: 10 k async/s, 500 sync/s/instance.

---

## Security and Reliability

* HTTPS everywhere; optional mutual TLS.
* HMAC‑sig headers; RBAC on config APIs.
* Circuit‑breakers, retries, health checks.
* Trace‑ID on every CDP → webhook span.
* PII redaction on DOM snapshots.

---

## Implementation Strategy

| Phase | Scope                                   | Weeks |
| ----- | --------------------------------------- | ----- |
| 1     | Registry, executor skeleton, flag gated | 4     |
| 2     | Rule engine, caching, perf              | 3     |
| 3     | Security, observability, docs           | 3     |
| 4     | Multi‑hook aggregation, ML hooks        | 4     |

---

## Monitoring and Observability

* **Metrics:** latency histograms, cache‑hit ratio, queue depth, error rate.
* **Dashboards:** Ops & business views.
* **Alerts:** Success < 95 %, P99 > budget, cache‑hit < 80 %.
* **Tracing:** CDP → webhook path annotated with outcome.

---

## Future Considerations

* Stateless scaling, distributed config store.
* ML‑driven rule optimisation.
* gRPC / event‑stream plugins.
* Microservice split: decision, executor, config, analytics.

---

## Appendix A

### Sample Rules

1. **Amazon Login Block** – blocks clicks on `*.amazon.com/ap/signin*` until captcha pass.
2. **Payment Form Guard** – blocks `Runtime.evaluate` when expression contains `.submit()` & URL includes `checkout`.

## Appendix B

| Priority Band | Purpose               |
| ------------- | --------------------- |
| 900–1000      | Emergency kill‑switch |
| 800–899       | Compliance / legal    |
| 600–799       | Security / fraud      |
| 400–599       | Performance tweaks    |
| 0–399         | Logging / analytics   |

## Appendix C

| Profile     | Timeout | Retries | Back‑off   |
| ----------- | ------- | ------- | ---------- |
| **Short**   | 3 s     | 1       | 1 s        |
| **Default** | 30 s    | 2       | 1, 3 s     |
| **Lenient** | 90 s    | 3       | 3, 6, 12 s |

---

## Glossary

* **CDP** – Chrome DevTools Protocol.
* **Hook** – External HTTP endpoint invoked by BrowserMux.
* **Blocking Rule** – YAML/JSON object defining when & how to block.

## References

* W3C Trace‑Context 1.0 Recommendation.
* Google Chrome DevTools Protocol v124.
* Hystrix Circuit Breaker Pattern (Netflix OSS).
