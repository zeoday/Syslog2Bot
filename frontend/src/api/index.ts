import { WebAPI } from './web'

const isWeb = typeof window !== 'undefined' && !(window as any).go

async function callAPI<T>(wailsFn: () => Promise<T>, webFn: () => Promise<T>): Promise<T> {
  if (isWeb) {
    return webFn()
  }
  return wailsFn()
}

export const API = {
  // Devices
  GetDevices: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetDevices()),
    () => WebAPI.GetDevices
  ),
  GetDevice: (id: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetDevice(id)),
    () => WebAPI.GetDevice(id)
  ),
  AddDevice: (device: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.AddDevice(device)),
    () => WebAPI.AddDevice(device)
  ),
  UpdateDevice: (device: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.UpdateDevice(device)),
    () => WebAPI.UpdateDevice(device)
  ),
  DeleteDevice: (id: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.DeleteDevice(id)),
    () => WebAPI.DeleteDevice(id)
  ),

  // Parse Templates
  GetParseTemplates: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetParseTemplates()),
    () => WebAPI.GetParseTemplates
  ),
  AddParseTemplate: (template: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.AddParseTemplate(template)),
    () => WebAPI.AddParseTemplate(template)
  ),
  UpdateParseTemplate: (template: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.UpdateParseTemplate(template)),
    () => WebAPI.UpdateParseTemplate(template)
  ),
  DeleteParseTemplate: (id: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.DeleteParseTemplate(id)),
    () => WebAPI.DeleteParseTemplate(id)
  ),

  // Filter Policies
  GetFilterPolicies: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetFilterPolicies()),
    () => WebAPI.GetFilterPolicies
  ),
  AddFilterPolicy: (policy: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.AddFilterPolicy(policy)),
    () => WebAPI.AddFilterPolicy(policy)
  ),
  UpdateFilterPolicy: (policy: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.UpdateFilterPolicy(policy)),
    () => WebAPI.UpdateFilterPolicy(policy)
  ),
  DeleteFilterPolicy: (id: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.DeleteFilterPolicy(id)),
    () => WebAPI.DeleteFilterPolicy(id)
  ),

  // Field Mapping Docs
  GetFieldMappingDocs: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetFieldMappingDocs()),
    () => WebAPI.GetFieldMappingDocs
  ),
  AddFieldMappingDoc: (doc: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.AddFieldMappingDoc(doc)),
    () => WebAPI.AddFieldMappingDoc(doc)
  ),
  UpdateFieldMappingDoc: (doc: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.UpdateFieldMappingDoc(doc)),
    () => WebAPI.UpdateFieldMappingDoc(doc)
  ),
  DeleteFieldMappingDoc: (id: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.DeleteFieldMappingDoc(id)),
    () => WebAPI.DeleteFieldMappingDoc(id)
  ),

  // Service
  StartService: (port: number, protocol: string) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.StartSyslogService(port, protocol)),
    () => WebAPI.StartService(port, protocol)
  ),
  StopService: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.StopSyslogService()),
    () => WebAPI.StopService()
  ),
  GetServiceStatus: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetServiceStatus()),
    () => WebAPI.GetServiceStatus
  ),
  GetSystemStats: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetSystemStats()),
    () => WebAPI.GetSystemStats
  ),
  GetSystemConfig: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetConfig()),
    () => WebAPI.GetSystemConfig
  ),
  SaveSystemConfig: (config: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.SaveConfig(config)),
    () => WebAPI.SaveSystemConfig(config)
  ),

  // Logs
  QueryLogs: (params: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.QueryLogs(params)),
    () => WebAPI.QueryLogs(params)
  ),
  CleanupLogs: (days: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.CleanupLogs(days)),
    () => WebAPI.CleanupLogs(days)
  ),

  // Test
  SendTestSyslog: (req: any) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.SendTestSyslog(req)),
    () => WebAPI.SendTestSyslog(req)
  ),
  GetLogTrace: (id: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetLogTraceInfo(id)),
    () => WebAPI.GetLogTrace(id)
  ),

  // Local IPs
  GetLocalIPs: () => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetLocalIPs()),
    () => WebAPI.GetLocalIPs
  ),

  // Stats
  GetFieldStats: (policyId: number, field: string, timeRange: string, topN: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetFieldStats(policyId, field, timeRange, topN)),
    () => WebAPI.GetFieldStats(policyId, field, timeRange, topN)
  ),
  GetAvailableStatsFields: (policyId: number) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.GetAvailableStatsFields(policyId)),
    () => WebAPI.GetAvailableStatsFields(policyId)
  ),

  // Import/Export
  ExportParseTemplates: (ids: number[]) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.ExportParseTemplates(ids)),
    () => WebAPI.ExportParseTemplates(ids)
  ),
  ExportFilterPolicies: (ids: number[]) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.ExportFilterPolicies(ids)),
    () => WebAPI.ExportFilterPolicies(ids)
  ),
  ImportParseTemplates: (jsonData: string) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.ImportParseTemplates(jsonData)),
    () => WebAPI.ImportParseTemplates(jsonData)
  ),
  ImportFilterPolicies: (jsonData: string) => callAPI(
    () => import('../../wailsjs/go/main/App').then(m => m.ImportFilterPolicies(jsonData)),
    () => WebAPI.ImportFilterPolicies(jsonData)
  ),
}
