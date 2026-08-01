DELETE FROM sys_parameters WHERE key LIKE 'scheduling.%'
   OR key LIKE 'orders.%' OR key LIKE 'pricing.%' OR key LIKE 'fulfilment.%'
   OR key LIKE 'auth.%'   OR key LIKE 'notify.%'  OR key LIKE 'finance.%'
   OR key LIKE 'company.%';
