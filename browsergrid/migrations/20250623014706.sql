-- Create "session_pools" table
CREATE TABLE "public"."session_pools" (
  "id" text NOT NULL,
  "name" text NULL,
  "description" text NULL,
  "browser" text NULL,
  "version" text NULL,
  "operating_system" text NULL,
  "screen" bytea NULL,
  "headless" boolean NULL,
  "min_size" bigint NULL,
  "max_size" bigint NULL,
  "current_size" bigint NULL,
  "available_size" bigint NULL,
  "max_idle_time" bigint NULL,
  "auto_scale" boolean NULL,
  "enabled" boolean NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "last_used_at" timestamptz NULL,
  "resource_limits" bytea NULL,
  "environment" jsonb NULL,
  PRIMARY KEY ("id")
);
-- Create "sessions" table
CREATE TABLE "public"."sessions" (
  "id" text NOT NULL,
  "browser" text NULL,
  "version" text NULL,
  "headless" boolean NULL,
  "operating_system" text NULL,
  "screen" bytea NULL,
  "proxy" bytea NULL,
  "resource_limits" bytea NULL,
  "environment" jsonb NULL,
  "status" text NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "expires_at" timestamptz NULL,
  "container_id" text NULL,
  "container_network" text NULL,
  "provider" text NULL,
  "webhooks_enabled" boolean NULL,
  "ws_endpoint" text NULL,
  "live_url" text NULL,
  "worker_id" text NULL,
  "work_pool_id" text NULL,
  "pool_id" text NULL,
  "is_pooled" boolean NULL,
  "claimed_at" timestamptz NULL,
  "claimed_by" text NULL,
  "available_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create "session_events" table
CREATE TABLE "public"."session_events" (
  "id" text NOT NULL,
  "session_id" text NULL,
  "event" text NULL,
  "data" jsonb NULL,
  "timestamp" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_session_events_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "session_metrics" table
CREATE TABLE "public"."session_metrics" (
  "id" text NOT NULL,
  "session_id" text NULL,
  "cpu_percent" numeric NULL,
  "memory_mb" numeric NULL,
  "network_rx_bytes" bigint NULL,
  "network_tx_bytes" bigint NULL,
  "timestamp" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_session_metrics_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "work_pools" table
CREATE TABLE "public"."work_pools" (
  "id" text NOT NULL,
  "name" text NULL,
  "description" text NULL,
  "provider" text NULL,
  "min_size" bigint NULL,
  "max_concurrency" bigint NULL,
  "max_idle_time" bigint NULL,
  "max_session_duration" bigint NULL,
  "auto_scale" boolean NULL,
  "paused" boolean NULL,
  "default_priority" bigint NULL,
  "queue_strategy" text NULL,
  "default_env" jsonb NULL,
  "default_image" text NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create "workers" table
CREATE TABLE "public"."workers" (
  "id" text NOT NULL,
  "pool_id" text NULL,
  "name" text NULL,
  "hostname" text NULL,
  "provider" text NULL,
  "max_slots" bigint NULL,
  "active" bigint NULL,
  "last_beat" timestamptz NULL,
  "started_at" timestamptz NULL,
  "paused" boolean NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_workers_pool" FOREIGN KEY ("pool_id") REFERENCES "public"."work_pools" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
