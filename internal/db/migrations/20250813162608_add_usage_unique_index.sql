-- Create index "usage_period_start_user_id_tunnel_id_domain" to table: "usages"
CREATE UNIQUE INDEX "usage_period_start_user_id_tunnel_id_domain" ON "public"."usages" ("period_start", "user_id", "tunnel_id", "domain");
