<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Upload, Download } from '@element-plus/icons-vue'
import { 
  GetFilterPolicies, 
  AddFilterPolicy, 
  UpdateFilterPolicy, 
  DeleteFilterPolicy,
  GetParseTemplates,
  GetDevices,
  GetDeviceGroups,
  GetFieldMappingDocs,
  ExportFilterPolicies,
  ImportFilterPolicies,
  SaveExportedFile
} from '../../wailsjs/go/main/App'
import { WebAPI } from '../api/web'

const isWeb = typeof window !== 'undefined' && !(window as any).go

interface FilterPolicy {
  id?: number
  name: string
  description: string
  deviceId: number
  deviceGroupId: number
  parseTemplateId: number
  conditions: string
  conditionLogic: string
  action: string
  priority: number
  isActive: boolean
  dedupEnabled: boolean
  dedupWindow: number
  dropUnmatched: boolean
}

interface FilterCondition {
  field: string
  operator: string
  value: string
}

const loading = ref(false)
const policies = ref<FilterPolicy[]>([])
const dialogVisible = ref(false)
const dialogTitle = ref('添加筛选策略')
const parseTemplates = ref<any[]>([])
const devices = ref<any[]>([])
const deviceGroups = ref<any[]>([])
const fieldMappingDocs = ref<any[]>([])
const selectedPolicies = ref<FilterPolicy[]>([])
const importDialogVisible = ref(false)
const importJsonContent = ref('')

const formData = ref<FilterPolicy>({
  name: '',
  description: '',
  deviceId: 0,
  deviceGroupId: 0,
  parseTemplateId: 0,
  conditions: '',
  conditionLogic: 'AND',
  action: 'keep',
  priority: 0,
  isActive: true,
  dedupEnabled: true,
  dedupWindow: 60,
  dropUnmatched: false
})

const conditions = ref<FilterCondition[]>([])
const newCondition = ref<FilterCondition>({
  field: '',
  operator: 'equals',
  value: ''
})

interface FieldMappingDoc {
  id: number
  name: string
  deviceType: string
  description: string
  fieldMappings: string
  isActive: boolean
}

const availableFields = computed(() => {
  if (!formData.value.parseTemplateId) return []
  const template = parseTemplates.value.find(t => t.id === formData.value.parseTemplateId)
  if (!template) return []

  const fields: { value: string; label: string }[] = []

  // 对于智能分隔符类型，从 fieldMapping 中提取 subTemplates
  if (template.parseType === 'smart_delimiter' && template.fieldMapping) {
    try {
      const fieldMappingData = JSON.parse(template.fieldMapping)

      // subTemplates 存储在 fieldMapping 字段内部
      const subTemplates = fieldMappingData.subTemplates
      if (!subTemplates) return []

      // 字段位置映射到字段名和中文名称的转换
      const fieldKeyMap: { [key: string]: { fieldName: string; chineseName: string } } = {
        'alertNameField': { fieldName: 'alertName', chineseName: '告警名称' },
        'attackIPField': { fieldName: 'attackIP', chineseName: '攻击IP' },
        'victimIPField': { fieldName: 'victimIP', chineseName: '受害IP' },
        'alertTimeField': { fieldName: 'alertTime', chineseName: '告警时间' },
        'severityField': { fieldName: 'severity', chineseName: '威胁等级' },
        'attackResultField': { fieldName: 'attackResult', chineseName: '攻击结果' }
      }

      // 收集所有子模板中配置的字段
      const configuredFields = new Set<string>()
      for (const subKey of Object.keys(subTemplates)) {
        const sub = subTemplates[subKey]
        for (const fieldKey of Object.keys(sub)) {
          const fieldInfo = fieldKeyMap[fieldKey]
          if (fieldInfo) {
            configuredFields.add(fieldKey)
          }
        }
      }

      // 使用 fieldKeyMap 中的中文名称
      for (const fieldKey of configuredFields) {
        const fieldInfo = fieldKeyMap[fieldKey]
        if (fieldInfo) {
          fields.push({ value: fieldInfo.fieldName, label: fieldInfo.chineseName + ' (' + fieldInfo.fieldName + ')' })
        }
      }
    } catch (e) {
      console.error('availableFields error:', e)
    }
  } else if (template.fieldMapping) {
    // 其他模板类型，从 fieldMapping 获取
    try {
      const mapping = JSON.parse(template.fieldMapping)
      for (const key of Object.keys(mapping)) {
        fields.push({ value: key, label: mapping[key] + ' (' + key + ')' })
      }
    } catch {
      // ignore
    }
  }

  return fields
})

const operators = [
  { value: 'equals', label: '等于' },
  { value: 'not_equals', label: '不等于' },
  { value: 'contains', label: '包含' },
  { value: 'not_contains', label: '不包含' },
  { value: 'in', label: '包含于' },
  { value: 'not_in', label: '不包含于' },
  { value: 'starts_with', label: '开头是' },
  { value: 'ends_with', label: '结尾是' },
  { value: 'regex', label: '正则匹配' },
  { value: 'exists', label: '字段存在' },
  { value: 'not_exists', label: '字段不存在' },
  { value: 'gt', label: '大于' },
  { value: 'gte', label: '大于等于' },
  { value: 'lt', label: '小于' },
  { value: 'lte', label: '小于等于' }
]

const actions = [
  { value: 'keep', label: '保留日志' },
  { value: 'discard', label: '丢弃日志' }
]

onMounted(async () => {
  await Promise.all([
    loadPolicies(),
    loadParseTemplates(),
    loadDevices(),
    loadDeviceGroups(),
    loadFieldMappingDocs()
  ])
})

async function loadPolicies() {
  loading.value = true
  try {
    if (isWeb) {
      policies.value = await WebAPI.GetFilterPolicies()
    } else {
      policies.value = await GetFilterPolicies()
    }
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

async function loadParseTemplates() {
  try {
    if (isWeb) {
      parseTemplates.value = await WebAPI.GetParseTemplates()
    } else {
      parseTemplates.value = await GetParseTemplates()
    }
  } catch (e) {
    console.error(e)
  }
}

async function loadDevices() {
  try {
    devices.value = await GetDevices()
  } catch (e) {
    console.error(e)
  }
}

async function loadDeviceGroups() {
  try {
    deviceGroups.value = await GetDeviceGroups()
  } catch (e) {
    console.error(e)
  }
}

async function loadFieldMappingDocs() {
  try {
    fieldMappingDocs.value = await GetFieldMappingDocs()
  } catch (e) {
    console.error(e)
  }
}

function handleAdd() {
  dialogTitle.value = '添加筛选策略'
  const maxPriority = policies.value.length > 0 ? Math.max(...policies.value.map(p => p.priority)) : 0
  formData.value = {
    name: '',
    description: '',
    deviceId: 0,
    deviceGroupId: 0,
    parseTemplateId: 0,
    conditions: '',
    conditionLogic: 'AND',
    action: 'keep',
    priority: maxPriority + 1,
    isActive: true,
    dedupEnabled: true,
    dedupWindow: 60,
    dropUnmatched: false
  }
  conditions.value = []
  dialogVisible.value = true
}

function handleSelectionChange(selection: FilterPolicy[]) {
  selectedPolicies.value = selection
}

function showImportDialog() {
  importJsonContent.value = ''
  importDialogVisible.value = true
}

async function handleImport() {
  if (!importJsonContent.value.trim()) {
    ElMessage.warning('请输入JSON内容')
    return
  }
  
  try {
    const result = await ImportFilterPolicies(importJsonContent.value)
    if (result.success) {
      ElMessage.success(result.message)
      importDialogVisible.value = false
      loadPolicies()
    } else {
      ElMessage.error(result.message)
    }
  } catch (e: any) {
    ElMessage.error('导入失败: ' + (e.message || '未知错误'))
  }
}

async function handleExport() {
  if (selectedPolicies.value.length === 0) {
    ElMessage.warning('请先选择要导出的策略')
    return
  }
  
  const ids = selectedPolicies.value.map(p => p.id).filter(Boolean) as number[]
  try {
    const jsonContent = await ExportFilterPolicies(ids)
    const timestamp = new Date().toISOString().slice(0, 10)
    const filename = `filter_policies_${timestamp}.json`
    
    const filePath = await SaveExportedFile(jsonContent, filename)
    ElMessage.success(`已导出到: ${filePath}`)
  } catch (e: any) {
    ElMessage.error('导出失败: ' + (e.message || '未知错误'))
  }
}

function handleEdit(row: FilterPolicy) {
  dialogTitle.value = '编辑筛选策略'
  formData.value = { ...row }
  if (row.conditions) {
    try {
      conditions.value = JSON.parse(row.conditions)
    } catch {
      conditions.value = []
    }
  } else {
    conditions.value = []
  }
  dialogVisible.value = true
}

async function handleDelete(row: FilterPolicy) {
  try {
    await ElMessageBox.confirm('确定要删除该筛选策略吗？', '提示', { type: 'warning' })
    if (isWeb) {
      await WebAPI.DeleteFilterPolicy(row.id!)
    } else {
      await DeleteFilterPolicy(row.id!)
    }
    ElMessage.success('删除成功')
    loadPolicies()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

function addCondition() {
  if (!newCondition.value.field) {
    ElMessage.warning('请输入字段名')
    return
  }
  conditions.value.push({ ...newCondition.value })
  newCondition.value = { field: '', operator: 'equals', value: '' }
}

function removeCondition(index: number) {
  conditions.value.splice(index, 1)
}

async function handleSubmit() {
  if (!formData.value.name) {
    ElMessage.warning('请填写策略名称')
    return
  }
  
  formData.value.conditions = JSON.stringify(conditions.value)
  
  try {
    if (formData.value.id) {
      if (isWeb) {
        await WebAPI.UpdateFilterPolicy(formData.value)
      } else {
        await UpdateFilterPolicy(formData.value)
      }
      ElMessage.success('更新成功')
    } else {
      if (isWeb) {
        await WebAPI.AddFilterPolicy(formData.value)
      } else {
        await AddFilterPolicy(formData.value)
      }
      ElMessage.success('添加成功')
    }
    dialogVisible.value = false
    loadPolicies()
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

function getParseTemplateName(id: number): string {
  const template = parseTemplates.value.find(t => t.id === id)
  return template ? template.name : '-'
}

function getDeviceName(id: number): string {
  if (id === 0) return '全部设备'
  const device = devices.value.find(d => d.id === id)
  return device ? device.name : '-'
}

function getActionText(action: string): string {
  return actions.find(a => a.value === action)?.label || action
}
</script>

<template>
  <div class="filter-policies-view">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span>筛选策略</span>
          <div class="header-actions">
            <el-button @click="showImportDialog">
              <el-icon><Upload /></el-icon>
              导入策略
            </el-button>
            <el-button @click="handleExport" :disabled="selectedPolicies.length === 0">
              <el-icon><Download /></el-icon>
              导出策略
            </el-button>
            <el-button type="primary" @click="handleAdd">
              <el-icon><Plus /></el-icon>
              添加策略
            </el-button>
          </div>
        </div>
      </template>
      
      <el-table :data="policies" v-loading="loading" stripe @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="50" />
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="策略名称" width="160" show-overflow-tooltip />
        <el-table-column label="解析模板" width="140" show-overflow-tooltip>
          <template #default="{ row }">
            {{ getParseTemplateName(row.parseTemplateId) }}
          </template>
        </el-table-column>
        <el-table-column label="设备" width="100" show-overflow-tooltip>
          <template #default="{ row }">
            {{ getDeviceName(row.deviceId) }}
          </template>
        </el-table-column>
        <el-table-column label="动作" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.action === 'keep' ? 'success' : 'danger'" size="small">
              {{ getActionText(row.action) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="70" align="center" />
        <el-table-column prop="description" label="描述" show-overflow-tooltip />
        <el-table-column label="状态" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.isActive ? 'success' : 'danger'" size="small">
              {{ row.isActive ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="800px">
      <el-form :model="formData" label-width="100px">
        <el-form-item label="策略名称" required>
          <el-input v-model="formData.name" placeholder="请输入策略名称" />
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="解析模板">
              <el-select v-model="formData.parseTemplateId" placeholder="选择解析模板" style="width: 100%" clearable>
                <el-option v-for="t in parseTemplates" :key="t.id" :label="t.name" :value="t.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="设备">
              <el-select v-model="formData.deviceId" placeholder="选择设备" style="width: 100%" clearable>
                <el-option :value="0" label="全部设备" />
                <el-option v-for="d in devices" :key="d.id" :label="d.name" :value="d.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        
        <el-form-item label="筛选条件">
          <div class="conditions-editor">
            <div class="condition-input">
              <el-select 
                v-model="newCondition.field" 
                placeholder="选择字段" 
                style="width: 200px" 
                filterable
                clearable
                :disabled="!formData.parseTemplateId"
              >
                <el-option 
                  v-for="f in availableFields" 
                  :key="f.value" 
                  :label="f.label" 
                  :value="f.value" 
                />
              </el-select>
              <el-select v-model="newCondition.operator" style="width: 120px">
                <el-option v-for="op in operators" :key="op.value" :label="op.label" :value="op.value" />
              </el-select>
              <el-input 
                v-model="newCondition.value" 
                :placeholder="newCondition.operator === 'in' || newCondition.operator === 'not_in' ? '多个值用逗号分隔' : '值'" 
                style="width: 200px" 
              />
              <el-button type="primary" @click="addCondition">添加</el-button>
              <span v-if="!formData.parseTemplateId" style="color: #999; margin-left: 10px; font-size: 12px;">请先选择解析模板</span>
            </div>
            
            <div v-if="conditions.length > 0" class="conditions-list">
              <div class="logic-toggle">
                <el-radio-group v-model="formData.conditionLogic" size="small">
                  <el-radio-button value="AND">满足全部</el-radio-button>
                  <el-radio-button value="OR">满足任一</el-radio-button>
                </el-radio-group>
              </div>
              <div v-for="(cond, idx) in conditions" :key="idx" class="condition-item">
                <span class="cond-field">{{ cond.field }}</span>
                <span class="cond-op">{{ operators.find(o => o.value === cond.operator)?.label }}</span>
                <span class="cond-value">{{ cond.value || '-' }}</span>
                <el-button type="danger" link size="small" @click="removeCondition(idx)">删除</el-button>
              </div>
            </div>
            
            <div class="condition-tips">
              <p><strong>操作符说明：</strong></p>
              <ul>
                <li><strong>包含于</strong>：字段值在指定的多个值中，多个值用逗号分隔。例如：威胁等级 包含于 "高危,危急"</li>
                <li><strong>不包含于</strong>：字段值不在指定的多个值中</li>
                <li><strong>正则匹配</strong>：使用正则表达式匹配</li>
              </ul>
            </div>
          </div>
        </el-form-item>
        
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="匹配动作">
              <el-select v-model="formData.action" style="width: 100%">
                <el-option v-for="a in actions" :key="a.value" :label="a.label" :value="a.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="未匹配丢弃">
              <el-switch 
                v-model="formData.dropUnmatched"
                active-text="丢弃"
                inactive-text="保留"
                inline-prompt
                class="drop-unmatched-switch"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="优先级">
              <el-input-number v-model="formData.priority" :min="0" :max="100" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        
        <el-form-item label="描述">
          <el-input v-model="formData.description" type="textarea" :rows="2" placeholder="请输入描述" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="formData.isActive" />
        </el-form-item>
        
        <el-form-item label="告警去重">
          <div class="dedup-config">
            <el-switch v-model="formData.dedupEnabled" />
            <span class="dedup-status">{{ formData.dedupEnabled ? '已启用' : '已禁用' }}</span>
            <el-input-number 
              v-if="formData.dedupEnabled" 
              v-model="formData.dedupWindow" 
              :min="10" 
              :max="3600" 
              :step="10"
              style="width: 120px; margin-left: 10px;"
            />
            <span v-if="formData.dedupEnabled" class="dedup-unit">秒</span>
          </div>
          <div class="dedup-desc">
            <el-alert type="info" :closable="false" show-icon>
              <template #title>
                启用后，相同告警在设定时间窗口内只推送一次。去重依据：设备ID + 策略ID + 攻击IP + 威胁类型 + 事件描述
              </template>
            </el-alert>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="导入筛选策略" width="600px">
      <el-form label-width="80px">
        <el-form-item label="JSON内容">
          <el-input
            v-model="importJsonContent"
            type="textarea"
            :rows="10"
            placeholder="粘贴JSON格式的筛选策略配置..."
          />
        </el-form-item>
        <el-form-item label="导入目录">
          <el-text type="info" size="small">
            也可将JSON文件放入程序根目录下的 templates/ 目录
          </el-text>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleImport">导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss" scoped>
.filter-policies-view {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    
    .header-actions {
      display: flex;
      gap: 10px;
    }
  }
  
  .conditions-editor {
    .condition-input {
      display: flex;
      gap: 10px;
      margin-bottom: 15px;
    }
    
    .conditions-list {
      background: var(--bg-secondary);
      border-radius: 8px;
      padding: 12px;
      
      .logic-toggle {
        margin-bottom: 10px;
      }
      
      .condition-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 8px 12px;
        background: var(--bg-card);
        border-radius: 6px;
        margin-bottom: 8px;
        
        .cond-field {
          color: var(--accent-color);
          font-weight: 500;
        }
        
        .cond-op {
          color: var(--text-secondary);
          font-size: 13px;
        }
        
        .cond-value {
          color: var(--text-primary);
          font-family: monospace;
          flex: 1;
          word-break: break-all;
        }
      }
    }
  }
  
  .condition-tips {
    margin-top: 12px;
    padding: 12px;
    background: var(--el-fill-color-light);
    border-radius: 6px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    
    p {
      margin: 0 0 8px 0;
    }
    
    ul {
      margin: 0;
      padding-left: 20px;
      
      li {
        margin-bottom: 4px;
      }
    }
  }
  
  .dedup-config {
    display: flex;
    align-items: center;
    gap: 8px;
    
    .dedup-status {
      color: var(--text-secondary);
      font-size: 13px;
    }
    
    .dedup-unit {
      color: var(--text-secondary);
      font-size: 13px;
    }
  }
  
  .drop-unmatched-switch {
    --el-switch-on-color: var(--accent-color);
    --el-switch-off-color: var(--text-muted);
  }
  
  .dedup-desc {
    margin-top: 10px;
  }
}
</style>
