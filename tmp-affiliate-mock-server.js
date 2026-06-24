const options = [
  ['PayAddress', ''],
  ['EpayId', ''],
  ['EpayKey', ''],
  ['Price', '7.3'],
  ['MinTopUp', '1'],
  ['CustomCallbackAddress', ''],
  ['PayMethods', '[]'],
  ['payment_setting.amount_options', '[10,20,50]'],
  ['payment_setting.amount_discount', '{}'],
  ['payment_setting.affiliate_rebate_enabled', 'true'],
  ['payment_setting.affiliate_rebate_rate', '12.5'],
  ['payment_setting.compliance_confirmed', 'true'],
  ['payment_setting.compliance_terms_version', 'v1'],
  ['payment_setting.compliance_confirmed_at', '1782180000'],
  ['payment_setting.compliance_confirmed_by', '1'],
  ['payment_setting.compliance_confirmed_ip', '127.0.0.1'],
  ['StripeApiSecret', ''],
  ['StripeWebhookSecret', ''],
  ['StripePriceId', ''],
  ['StripeUnitPrice', '8'],
  ['StripeMinTopUp', '1'],
  ['StripePromotionCodesEnabled', 'false'],
  ['CreemApiKey', ''],
  ['CreemWebhookSecret', ''],
  ['CreemTestMode', 'false'],
  ['CreemProducts', '[]'],
  ['WaffoEnabled', 'false'],
  ['WaffoApiKey', ''],
  ['WaffoPrivateKey', ''],
  ['WaffoPublicCert', ''],
  ['WaffoSandboxPublicCert', ''],
  ['WaffoSandboxApiKey', ''],
  ['WaffoSandboxPrivateKey', ''],
  ['WaffoSandbox', 'false'],
  ['WaffoMerchantId', ''],
  ['WaffoCurrency', 'USD'],
  ['WaffoUnitPrice', '1'],
  ['WaffoMinTopUp', '1'],
  ['WaffoNotifyUrl', ''],
  ['WaffoReturnUrl', ''],
  ['WaffoPayMethods', '[]'],
  ['WaffoPancakeMerchantID', ''],
  ['WaffoPancakePrivateKey', ''],
  ['WaffoPancakeReturnURL', ''],
].map(([key, value]) => ({ key, value }))

const headers = {
  'content-type': 'application/json',
  'access-control-allow-origin': '*',
  'access-control-allow-headers': '*',
  'access-control-allow-methods': 'GET,PUT,POST,OPTIONS',
}

Bun.serve({
  hostname: '127.0.0.1',
  port: 4500,
  fetch(req) {
    const url = new URL(req.url)
    if (req.method === 'OPTIONS') return new Response('{}', { headers })
    if (url.pathname.startsWith('/api/user/self')) {
      return Response.json({
        success: true,
        data: {
          id: 1,
          username: 'admin',
          display_name: 'admin',
          role: 100,
          group: 'default',
        },
      }, { headers })
    }
    if (url.pathname.startsWith('/api/status')) {
      return Response.json({
        success: true,
        data: {
          Status: true,
          SystemName: 'new-api',
          ServerAddress: 'http://127.0.0.1:4500',
          SidebarModulesAdmin: {},
          SidebarModulesUser: {},
        },
      }, { headers })
    }
    if (url.pathname.startsWith('/api/option/payment_compliance')) {
      return Response.json({ success: true, message: 'ok' }, { headers })
    }
    if (url.pathname.startsWith('/api/option')) {
      return Response.json({ success: true, data: options }, { headers })
    }
    return Response.json({ success: true, data: [] }, { headers })
  },
})

await new Promise(() => {})
