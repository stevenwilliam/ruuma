DELETE FROM sys_parameters WHERE key IN (
  'company.whatsapp_enabled',
  'company.whatsapp_number',
  'company.whatsapp_message_id',
  'company.whatsapp_message_en'
);
