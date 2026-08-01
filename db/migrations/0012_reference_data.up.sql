-- 0012 reference data: the sys_parameters defaults from BR-2.9.1.
--
-- These are group defaults; a store overrides any of them through
-- store_parameters (BR-1.4.4). Nothing here is a hard-coded constant in Go —
-- the service reads these rows at runtime (BR-1.4.1).
--
-- PostgreSQL 18 provides uuidv7() natively, which matches BR-1.2.1.

INSERT INTO sys_parameters (id, key, value, data_type, description, is_store_overridable) VALUES
  (uuidv7(),'scheduling.slot_length_minutes','30','int','Length of one fulfilment slot in minutes',true),
  (uuidv7(),'scheduling.max_orders_per_slot','12','int','Capacity axis 1: orders per slot',true),
  (uuidv7(),'scheduling.max_kitchen_units_per_slot','60','int','Capacity axis 2: kitchen units per slot',true),
  (uuidv7(),'scheduling.lead_time_minutes','90','int','Earliest bookable distance from now',true),
  (uuidv7(),'scheduling.cutoff_minutes','60','int','A slot closes this long before it starts',true),
  (uuidv7(),'scheduling.max_advance_days','14','int','Furthest bookable date from today',true),
  (uuidv7(),'scheduling.cancel_cutoff_minutes','120','int','Customer self-cancel limit before slot start',true),
  (uuidv7(),'orders.auto_cancel_minutes','0','int','0 = no auto-cancel in phase 1 (D25)',false),
  (uuidv7(),'orders.max_unpaid_per_customer','2','int','Concurrent unpaid orders per customer; 0 = unlimited',false),
  (uuidv7(),'pricing.tax_bps','1000','int','PB1 restaurant tax in basis points (10%)',true),
  (uuidv7(),'pricing.service_charge_bps','0','int','Service charge in basis points',true),
  (uuidv7(),'pricing.tax_inclusive','false','bool','Menu prices already include tax',false),
  (uuidv7(),'pricing.quote_ttl_minutes','15','int','How long a cart quote stays valid',false),
  (uuidv7(),'fulfilment.delivery_enabled','false','bool','Phase-2 delivery switch (D16)',false),
  (uuidv7(),'auth.otp_ttl_minutes','5','int','OTP lifetime',false),
  (uuidv7(),'auth.otp_max_attempts','5','int','OTP attempts before lockout',false),
  (uuidv7(),'auth.access_token_minutes','15','int','Access token lifetime',false),
  (uuidv7(),'auth.refresh_token_days','30','int','Refresh token lifetime',false),
  (uuidv7(),'auth.provider_google_enabled','false','bool','Enable Google sign-in (needs credentials)',false),
  (uuidv7(),'auth.provider_instagram_enabled','false','bool','Enable Instagram sign-in (needs credentials)',false),
  (uuidv7(),'notify.provider','log','string','waha | meta_cloud | log',false),
  (uuidv7(),'notify.event.order_received_enabled','true','bool','Send order-received message',false),
  (uuidv7(),'notify.event.payment_verified_enabled','true','bool','Send payment-verified message',false),
  (uuidv7(),'notify.event.order_ready_enabled','true','bool','Send order-ready message',false),
  (uuidv7(),'notify.event.slot_reminder_enabled','true','bool','Send pre-slot reminder',false),
  (uuidv7(),'notify.slot_reminder_minutes','60','int','How long before the slot to remind',false),
  (uuidv7(),'finance.verification_sla_minutes','60','int','Ageing alarm for pending verifications',false),
  (uuidv7(),'company.name','Ruuma Eatery','string','Legal/trading name shown to customers',false),
  (uuidv7(),'company.phone','+6221000000','string','Group contact phone',false),
  (uuidv7(),'company.email','halo@ruuma.id','string','Group contact email',false),
  (uuidv7(),'company.address','Jakarta, Indonesia','string','Group address',false);

-- Notification templates (BR-2.10.5). {{placeholders}} are filled by notifysvc.
INSERT INTO sys_parameters (id, key, value, data_type, description) VALUES
  (uuidv7(),'notify.template.order_received.id',
   'Halo {{name}}, pesanan {{code}} di {{store}} sudah kami terima. Silakan transfer *Rp {{amount_due}}* (nominal termasuk kode unik {{unique_code}}) ke {{bank}} a/n {{account_name}} no. {{account_number}}, lalu unggah bukti transfer. Jadwal ambil: {{slot}}.',
   'string','WhatsApp: order received (ID)'),
  (uuidv7(),'notify.template.order_received.en',
   'Hi {{name}}, we have received order {{code}} at {{store}}. Please transfer *Rp {{amount_due}}* (the amount includes unique code {{unique_code}}) to {{bank}}, account {{account_name}} no. {{account_number}}, then upload your transfer proof. Pickup: {{slot}}.',
   'string','WhatsApp: order received (EN)'),
  (uuidv7(),'notify.template.payment_verified.id',
   'Pembayaran pesanan {{code}} sudah kami verifikasi. Pesanan Anda diproses dan siap diambil di {{store}} pada {{slot}}.',
   'string','WhatsApp: payment verified (ID)'),
  (uuidv7(),'notify.template.payment_verified.en',
   'Payment for order {{code}} is verified. We are preparing it for pickup at {{store}} on {{slot}}.',
   'string','WhatsApp: payment verified (EN)'),
  (uuidv7(),'notify.template.order_ready.id',
   'Pesanan {{code}} sudah siap diambil di {{store}} ({{address}}). Tunjukkan kode {{code}} di kasir.',
   'string','WhatsApp: order ready (ID)'),
  (uuidv7(),'notify.template.order_ready.en',
   'Order {{code}} is ready for pickup at {{store}} ({{address}}). Show code {{code}} at the counter.',
   'string','WhatsApp: order ready (EN)'),
  (uuidv7(),'notify.template.slot_reminder.id',
   'Pengingat: pesanan {{code}} dijadwalkan diambil di {{store}} pada {{slot}}.',
   'string','WhatsApp: pre-slot reminder (ID)'),
  (uuidv7(),'notify.template.slot_reminder.en',
   'Reminder: order {{code}} is scheduled for pickup at {{store}} on {{slot}}.',
   'string','WhatsApp: pre-slot reminder (EN)');
