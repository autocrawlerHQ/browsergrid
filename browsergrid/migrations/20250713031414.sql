-- Modify "profiles" table
ALTER TABLE "public"."profiles" ALTER COLUMN "id" DROP DEFAULT, ALTER COLUMN "storage_backend" DROP DEFAULT, ALTER COLUMN "metadata" DROP DEFAULT, ALTER COLUMN "created_at" DROP DEFAULT, ALTER COLUMN "updated_at" DROP DEFAULT;
