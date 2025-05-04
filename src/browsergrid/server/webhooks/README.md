# BrowserFleet Webhooks

## Overview

Webhooks in BrowserFleet provide a powerful way to integrate with external systems by sending real-time notifications when specific Chrome DevTools Protocol (CDP) events occur. This enables you to build automated workflows, trigger external processes, and connect BrowserFleet to your existing tools and services.

## What are Webhooks?

Webhooks are user-defined HTTP callbacks that are triggered by specific events. When a subscribed event occurs, BrowserFleet makes an HTTP request to the webhook URL you've configured, delivering information about the event that just occurred. This enables:

- Real-time notifications about browser events
- Integration with third-party services
- Automation of workflows based on browser activity
- Collection of custom analytics and metrics 

webhooks can be configured to run before or after events are sent to connected user clients

## How BrowserFleet Webhooks Work

### CDP Event Integration

BrowserFleet webhooks specifically target Chrome DevTools Protocol (CDP) events. The CDP is a protocol used by Chrome (and other Chromium-based browsers) to expose browser internals for debugging and automation.

### Key Components

1. **Event Pattern Matching**: Define specific CDP events to monitor (e.g., `Page.navigate`, `Network.responseReceived`)
2. **Timing Options**: Choose to trigger webhooks either before or after CDP events are processed
3. **Customizable Endpoints**: Configure your own HTTP endpoints to receive webhook data
4. **Reliability Features**: Includes timeout handling, retry mechanisms, and execution tracking

## Creating and Managing Webhooks

### Webhook Configuration

A webhook in BrowserFleet requires:

- **Name**: A descriptive name for the webhook
- **Event Pattern**: Specifies which CDP events to monitor
  - Method: The CDP method name (e.g., `Page.navigate`)
  - Parameter Filters: Optional filters to match specific event parameters
- **Timing**: Whether to trigger before or after the event is processed
- **Webhook URL**: The HTTP(S) endpoint that will receive the webhook payload
- **Headers**: Custom HTTP headers to include in the webhook request
- **Timeout**: Maximum time to wait for a response (default: 10 seconds)
- **Max Retries**: Number of retry attempts on failure (default: 3)

### Webhook Execution Tracking

BrowserFleet tracks every webhook execution, providing:

- Execution status (pending, success, failed, timeout)
- Response details (status code, body)
- Performance metrics (execution time)
- Error information for troubleshooting

## Webhook Payload

When a webhook is triggered, BrowserFleet sends a JSON payload to your configured endpoint containing:

```json
{
  "webhook_id": "uuid",
  "session_id": "uuid",
  "cdp_event": "Domain.method",
  "event_data": {
    // Raw CDP event data
  },
  "timing": "before_event|after_event",
  "timestamp": "ISO-8601 timestamp"
}
```

## Best Practices

1. **Keep Webhook Handlers Fast**: Implement quick-responding webhook handlers to avoid timeouts
2. **Implement Idempotency**: Design your webhook handlers to safely handle duplicate events
3. **Security Considerations**: 
   - Use HTTPS endpoints
   - Consider implementing webhook signature validation
   - Keep webhook URLs confidential

## Example Use Cases

- Trigger CI/CD pipelines when specific browser events occur
- Send notifications when errors are detected in a browser session
- Log browser activity to external monitoring systems
- Implement custom analytics for user behavior
- Synchronize browser state with other applications

## Troubleshooting

Common issues and their solutions:

1. **Webhook Not Firing**: Verify the event pattern is correctly configured and matches the expected CDP events
2. **Timeouts**: Ensure your webhook handler responds within the configured timeout period
3. **Failed Executions**: Check the webhook execution logs for detailed error messages 