-- 0016 WhatsApp contact: the floating "chat with us" button on the customer
-- site (BR-1.4.5).
--
-- The filename must keep the word "parameters": db/embed.go DataMigrations()
-- selects reference-data migrations by matching "reference_data" or
-- "parameters" in the name, and test/testenv replays exactly those after it
-- TRUNCATEs. Named anything else, these rows vanish in the test database and
-- the suite quietly exercises compiled fallbacks instead.
--
-- The number is a sys_parameters row rather than a constant because a
-- restaurant changes the number it answers on far more often than it deploys
-- (CLAUDE.md §7, BR-1.4.1). Same reason the greeting is a parameter: it is
-- customer-facing copy, and copy is not a code change.
--
-- Stored in E.164 without punctuation or a leading +, which is the form
-- https://wa.me/<number> expects. 62 is Indonesia; a leading 0 is the local
-- trunk prefix and must be dropped, so 0812... is 62812....
--
-- is_store_overridable is false, matching company.phone and company.email:
-- this button is site chrome and shows on pages with no store context at all
-- (credits, sign-in). A per-store number would have nothing to resolve
-- against there. If a second outlet ever needs its own number, flip this and
-- resolve it from the store the customer is ordering from.

INSERT INTO sys_parameters (id, key, value, data_type, description, is_store_overridable) VALUES
  (uuidv7(),'company.whatsapp_enabled','true','bool',
   'Show the floating WhatsApp button on the customer site',false),
  (uuidv7(),'company.whatsapp_number','628176315568','string',
   'WhatsApp number in E.164 without + or spaces, e.g. 6281234567890',false),
  (uuidv7(),'company.whatsapp_message_id','Halo Ruuma, saya ingin bertanya tentang pesanan saya.','string',
   'Prefilled WhatsApp greeting (ID)',false),
  (uuidv7(),'company.whatsapp_message_en','Hello Ruuma, I have a question about my order.','string',
   'Prefilled WhatsApp greeting (EN)',false)
ON CONFLICT (key) DO NOTHING;
