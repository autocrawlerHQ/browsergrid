/**
 * Generated by orval v7.10.0 🍺
 * Do not edit manually.
 * BrowserGrid API
 * BrowserGrid is a distributed browser automation platform that provides scalable browser sessions and worker pool management.
 * OpenAPI spec version: 1.0
 */

export type GetApiV1SessionsIdEventsEventType = typeof GetApiV1SessionsIdEventsEventType[keyof typeof GetApiV1SessionsIdEventsEventType];


// eslint-disable-next-line @typescript-eslint/no-redeclare
export const GetApiV1SessionsIdEventsEventType = {
  session_created: 'session_created',
  resource_allocated: 'resource_allocated',
  session_starting: 'session_starting',
  container_started: 'container_started',
  browser_started: 'browser_started',
  session_available: 'session_available',
  session_claimed: 'session_claimed',
  session_assigned: 'session_assigned',
  session_ready: 'session_ready',
  session_active: 'session_active',
  session_idle: 'session_idle',
  heartbeat: 'heartbeat',
  pool_added: 'pool_added',
  pool_removed: 'pool_removed',
  pool_drained: 'pool_drained',
  session_completed: 'session_completed',
  session_expired: 'session_expired',
  session_timed_out: 'session_timed_out',
  session_terminated: 'session_terminated',
  startup_failed: 'startup_failed',
  browser_crashed: 'browser_crashed',
  container_crashed: 'container_crashed',
  resource_exhausted: 'resource_exhausted',
  network_error: 'network_error',
  status_changed: 'status_changed',
  config_updated: 'config_updated',
  health_check: 'health_check',
} as const;
