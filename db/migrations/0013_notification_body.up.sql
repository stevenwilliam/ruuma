-- 0013 store the rendered message body with the queued notification.
--
-- The template lives in sys_parameters (BR-2.10.5), but rendering happens when
-- the event occurs, not when the worker later picks the row up: an order's
-- amounts and slot are facts at queue time, and re-rendering hours later from
-- changed parameters would send a message that no longer matches the order.
ALTER TABLE notifications ADD COLUMN body TEXT NOT NULL DEFAULT '';
