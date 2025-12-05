-- Modify "tunnels" table
ALTER TABLE "public"."tunnels" ADD COLUMN "dns_propagation_status" character varying NOT NULL DEFAULT 'verified';
