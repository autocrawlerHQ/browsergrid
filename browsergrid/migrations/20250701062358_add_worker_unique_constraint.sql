-- Create "workers" table unique constraint migration
-- First, clean up duplicate workers by keeping only the most recent one per (pool_id, hostname)
WITH ranked_workers AS (
  SELECT id, 
         pool_id, 
         hostname,
         ROW_NUMBER() OVER (
           PARTITION BY pool_id, hostname 
           ORDER BY last_beat DESC, started_at DESC
         ) as rn
  FROM workers
)
DELETE FROM workers 
WHERE id IN (
  SELECT id FROM ranked_workers WHERE rn > 1
);

-- Now create the unique index
CREATE UNIQUE INDEX "idx_workers_pool_hostname" ON "public"."workers" ("pool_id", "hostname");
