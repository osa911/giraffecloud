-- Modify "tunnels" table
ALTER TABLE "public"."tunnels" DROP COLUMN "token", ADD COLUMN "target_host" character varying NOT NULL DEFAULT 'localhost';
-- Create index "tunnel_user_id_target_host_target_port" to table: "tunnels"
CREATE UNIQUE INDEX "tunnel_user_id_target_host_target_port" ON "public"."tunnels" ("user_id", "target_host", "target_port");
