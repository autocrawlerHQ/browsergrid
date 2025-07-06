-- Migration: Remove worker architecture components
-- This removes the database tables and columns that were used for the old
-- worker registration system, replaced by Asynq task queue

-- Remove worker-related columns from sessions table
ALTER TABLE sessions DROP COLUMN IF EXISTS worker_id;

-- Drop the workers table entirely
DROP TABLE IF EXISTS workers CASCADE;

-- Remove any indexes related to workers
DROP INDEX IF EXISTS idx_sessions_worker_id;
DROP INDEX IF EXISTS idx_workers_pool_id;
DROP INDEX IF EXISTS idx_workers_pool_hostname; 