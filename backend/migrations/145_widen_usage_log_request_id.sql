-- Widen usage_logs.request_id for WebSocket per-turn billing request IDs.
--
-- The WS billing key includes the client request id, turn number, and upstream
-- response id. That is intentionally longer than the original 64-byte
-- upstream request id shape, and must match usage_billing_dedup.request_id.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

ALTER TABLE IF EXISTS usage_logs
  ALTER COLUMN request_id TYPE VARCHAR(255);
