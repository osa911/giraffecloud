-- Rename "is_active" column to "is_enabled" in "tunnels" table
ALTER TABLE "public"."tunnels" RENAME COLUMN "is_active" TO "is_enabled";
