-- Create "deployments" table
CREATE TABLE "public"."deployments" (
  "id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "version" text NOT NULL,
  "runtime" text NOT NULL,
  "package_url" text NOT NULL,
  "package_hash" text NOT NULL,
  "config" jsonb NULL,
  "status" text NOT NULL DEFAULT 'active',
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_deployments_name" to table: "deployments"
CREATE INDEX "idx_deployments_name" ON "public"."deployments" ("name");
-- Create "deployment_runs" table
CREATE TABLE "public"."deployment_runs" (
  "id" uuid NOT NULL,
  "deployment_id" uuid NOT NULL,
  "session_id" uuid NULL,
  "status" text NOT NULL DEFAULT 'pending',
  "started_at" timestamptz NOT NULL,
  "completed_at" timestamptz NULL,
  "output" jsonb NULL,
  "error" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_deployments_runs" FOREIGN KEY ("deployment_id") REFERENCES "public"."deployments" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_deployment_runs_deployment_id" to table: "deployment_runs"
CREATE INDEX "idx_deployment_runs_deployment_id" ON "public"."deployment_runs" ("deployment_id");
-- Create index "idx_deployment_runs_session_id" to table: "deployment_runs"
CREATE INDEX "idx_deployment_runs_session_id" ON "public"."deployment_runs" ("session_id");
