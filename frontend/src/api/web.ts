const API_BASE = ''

async function fetchAPI(endpoint: string, options: RequestInit = {}) {
  const response = await fetch(`${API_BASE}/api/${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  })
  const data = await response.json()
  if (!response.ok) {
    throw new Error(data.error || `HTTP ${response.status}`)
  }
  return data
}

export const WebAPI = {
  // Devices
  GetDevices: () => fetchAPI('devices'),
  GetDevice: (id: number) => fetchAPI(`device/${id}`),
  AddDevice: (device: any) => fetchAPI('device/0', { method: 'POST', body: JSON.stringify(device) }),
  UpdateDevice: (device: any) => fetchAPI(`device/${device.id}`, { method: 'PUT', body: JSON.stringify(device) }),
  DeleteDevice: (id: number) => fetchAPI(`device/${id}`, { method: 'DELETE' }),

  // Device Groups
  GetDeviceGroups: () => fetchAPI('device-groups'),
  AddDeviceGroup: (group: any) => fetchAPI('device-group/0', { method: 'POST', body: JSON.stringify(group) }),
  UpdateDeviceGroup: (group: any) => fetchAPI(`device-group/${group.id}`, { method: 'PUT', body: JSON.stringify(group) }),
  DeleteDeviceGroup: (id: number) => fetchAPI(`device-group/${id}`, { method: 'DELETE' }),

  // Parse Templates
  GetParseTemplates: () => fetchAPI('parse-templates'),
  AddParseTemplate: (template: any) => fetchAPI('parse-template/0', { method: 'POST', body: JSON.stringify(template) }),
  UpdateParseTemplate: (template: any) => fetchAPI(`parse-template/${template.id}`, { method: 'PUT', body: JSON.stringify(template) }),
  DeleteParseTemplate: (id: number) => fetchAPI(`parse-template/${id}`, { method: 'DELETE' }),

  // Filter Policies
  GetFilterPolicies: () => fetchAPI('filter-policies'),
  AddFilterPolicy: (policy: any) => fetchAPI('filter-policy/0', { method: 'POST', body: JSON.stringify(policy) }),
  UpdateFilterPolicy: (policy: any) => fetchAPI(`filter-policy/${policy.id}`, { method: 'PUT', body: JSON.stringify(policy) }),
  DeleteFilterPolicy: (id: number) => fetchAPI(`filter-policy/${id}`, { method: 'DELETE' }),

  // Alert Policies
  GetAlertPolicies: () => fetchAPI('alert-policies'),
  AddAlertPolicy: (policy: any) => fetchAPI('alert-policy/0', { method: 'POST', body: JSON.stringify(policy) }),
  UpdateAlertPolicy: (policy: any) => fetchAPI(`alert-policy/${policy.id}`, { method: 'PUT', body: JSON.stringify(policy) }),
  DeleteAlertPolicy: (id: number) => fetchAPI(`alert-policy/${id}`, { method: 'DELETE' }),

  // Alert Rules
  GetAlertRules: (robotId: number) => fetchAPI(`alert-rules/${robotId}`),
  AddAlertRule: (rule: any) => fetchAPI('alert-rule/0', { method: 'POST', body: JSON.stringify(rule) }),
  UpdateAlertRule: (rule: any) => fetchAPI(`alert-rule/${rule.id}`, { method: 'PUT', body: JSON.stringify(rule) }),
  DeleteAlertRule: (id: number) => fetchAPI(`alert-rule/${id}`, { method: 'DELETE' }),
  DeleteAlertRulesByRobotID: (robotId: number) => fetchAPI(`alert-rules/robot/${robotId}`, { method: 'DELETE' }),

  // Robots
  GetRobots: () => fetchAPI('robots'),
  AddRobot: (robot: any) => fetchAPI('robot/0', { method: 'POST', body: JSON.stringify(robot) }),
  UpdateRobot: (robot: any) => fetchAPI(`robot/${robot.id}`, { method: 'PUT', body: JSON.stringify(robot) }),
  DeleteRobot: (id: number) => fetchAPI(`robot/${id}`, { method: 'DELETE' }),
  TestRobot: (robot: any) => fetchAPI('test-robot', { method: 'POST', body: JSON.stringify(robot) }),

  // Output Templates
  GetOutputTemplates: () => fetchAPI('output-templates'),
  AddOutputTemplate: (template: any) => fetchAPI('output-template/0', { method: 'POST', body: JSON.stringify(template) }),
  UpdateOutputTemplate: (template: any) => fetchAPI(`output-template/${template.id}`, { method: 'PUT', body: JSON.stringify(template) }),
  DeleteOutputTemplate: (id: number) => fetchAPI(`output-template/${id}`, { method: 'DELETE' }),

  // Field Mapping Docs
  GetFieldMappingDocs: () => fetchAPI('field-mapping-docs'),
  AddFieldMappingDoc: (doc: any) => fetchAPI('field-mapping-doc/0', { method: 'POST', body: JSON.stringify(doc) }),
  UpdateFieldMappingDoc: (doc: any) => fetchAPI(`field-mapping-doc/${doc.id}`, { method: 'PUT', body: JSON.stringify(doc) }),
  DeleteFieldMappingDoc: (id: number) => fetchAPI(`field-mapping-doc/${id}`, { method: 'DELETE' }),

  // Logs
  GetLogs: (params: any) => fetchAPI(`logs?page=${params.page || 1}&pageSize=${params.pageSize || 50}&deviceId=${params.deviceId || 0}`),
  QueryLogs: (params: any) => fetchAPI(`logs?page=${params.page || 1}&pageSize=${params.pageSize || 50}`),
  CleanupLogs: (days: number) => fetchAPI('logs/cleanup', { method: 'POST', body: JSON.stringify({ days }) }),

  // Service
  GetServiceStatus: () => fetchAPI('service/status'),
  StartService: (port: number, protocol: string) => fetchAPI('service/start', { method: 'POST', body: JSON.stringify({ port, protocol }) }),
  StopService: () => fetchAPI('service/stop', { method: 'POST' }),

  // Config
  GetSystemConfig: () => fetchAPI('config'),
  SaveSystemConfig: (config: any) => fetchAPI('config', { method: 'PUT', body: JSON.stringify(config) }),

  // Stats
  GetSystemStats: () => fetchAPI('stats'),
  GetFieldStats: (params: any) => fetchAPI('field-stats', { method: 'POST', body: JSON.stringify(params) }),
  GetAvailableStatsFields: (policyId: number) => fetchAPI(`available-stats-fields/${policyId}`),

  // Test
  SendTestSyslog: (req: any) => fetchAPI('test-syslog', { method: 'POST', body: JSON.stringify(req) }),
  TestParseTemplate: (req: any) => fetchAPI('test-parse', { method: 'POST', body: JSON.stringify(req) }),
  GetLogTrace: (id: number) => fetchAPI(`log-trace/${id}`),
  TestSyslogForward: (host: string, port: number, protocol: string, format: string) => fetchAPI(`test-syslog-forward?host=${host}&port=${port}&protocol=${protocol}&format=${format}`),

  // Local IPs
  GetLocalIPs: () => fetchAPI('local-ips'),

  // Export/Import
  ExportParseTemplates: (ids: number[]) => fetchAPI('export/parse-templates', { method: 'POST', body: JSON.stringify(ids) }),
  ExportFilterPolicies: (ids: number[]) => fetchAPI('export/filter-policies', { method: 'POST', body: JSON.stringify(ids) }),
  ImportParseTemplates: (jsonData: string) => fetchAPI('import/parse-templates', { method: 'POST', body: jsonData }),
  ImportFilterPolicies: (jsonData: string) => fetchAPI('import/filter-policies', { method: 'POST', body: jsonData }),
}

export function isWebMode(): boolean {
  return typeof window !== 'undefined' && !!(window as any).webMode
}
