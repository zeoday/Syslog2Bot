<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { GetFilterPolicies, GetFieldStats, GetAvailableStatsFields } from '../../wailsjs/go/main/App'
import { WebAPI } from '../api/web'

const isWeb = typeof window !== 'undefined' && !(window as any).go

interface FilterPolicy {
  id: number
  name: string
  description: string
  isActive: boolean
}

interface StatsItem {
  value: string
  count: number
  percent: string
  lastSeen: string
}interface FieldStatsResult {
  field: string
  totalLogs: number
  uniqueCount: number
  items: StatsItem[]
}

interface StatsField {
  name: string
  displayName: string
}

const loading = ref(false)
const filterPolicies = ref<FilterPolicy[]>([])
const availableFields = ref<StatsField[]>([])

const selectedPolicyId = ref<number | undefined>(undefined)
const timeRange = ref<string>('24h')
const selectedField = ref<string>('')
const topN = ref<number>(10)

const statsResult = ref<FieldStatsResult | null>(null)

const timeRangeOptions = [
  { value: '1h', label: '最近1小时' },
  { value: '6h', label: '最近6小时' },
  { value: '24h', label: '最近24小时' },
  { value: '7d', label: '最近7天' },
  { value: '30d', label: '最近30天' },
  { value: 'custom', label: '自定义' }
]

const customStartTime = ref('')
const customEndTime = ref('')

const topNOptions = [
  { value: 5, label: 'Top 5' },
  { value: 10, label: 'Top 10' },
  { value: 20, label: 'Top 20' },
  { value: 50, label: 'Top 50' },
  { value: 100, label: 'Top 100' }
]

const maxCount = computed(() => {
  if (!statsResult.value || statsResult.value.items.length === 0) return 0
  return Math.max(...statsResult.value.items.map(i => i.count))
})

onMounted(async () => {
  await loadFilterPolicies()
})

async function loadFilterPolicies() {
  try {
    let policies
    if (isWeb) {
      policies = await WebAPI.GetFilterPolicies()
    } else {
      policies = await GetFilterPolicies()
    }
    filterPolicies.value = policies || []
  } catch (e) {
    console.error(e)
  }
}

watch(selectedPolicyId, async (newVal) => {
  if (newVal) {
    try {
      let fields
      if (isWeb) {
        fields = await WebAPI.GetAvailableStatsFields(newVal)
      } else {
        fields = await GetAvailableStatsFields(newVal)
      }
      availableFields.value = fields || []
      if (fields && fields.length > 0 && !selectedField.value) {
        selectedField.value = fields[0].name
      }
    } catch (e) {
      console.error(e)
    }
  } else {
    availableFields.value = []
  }
})

function getTimeRange(): { startTime: string, endTime: string } {
  const now = new Date()
  let startTime: Date
  
  if (timeRange.value === 'custom') {
    return {
      startTime: customStartTime.value,
      endTime: customEndTime.value
    }
  }
  
  switch (timeRange.value) {
    case '1h':
      startTime = new Date(now.getTime() - 60 * 60 * 1000)
      break
    case '6h':
      startTime = new Date(now.getTime() - 6 * 60 * 60 * 1000)
      break
    case '24h':
      startTime = new Date(now.getTime() - 24 * 60 * 60 * 1000)
      break
    case '7d':
      startTime = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
      break
    case '30d':
      startTime = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
      break
    default:
      startTime = new Date(now.getTime() - 24 * 60 * 60 * 1000)
  }
  
  return {
    startTime: formatDateTime(startTime),
    endTime: formatDateTime(now)
  }
}

function formatDateTime(date: Date): string {
  return date.toISOString().slice(0, 19).replace('T', ' ')
}

async function handleQuery() {
  if (!selectedField.value) {
    ElMessage.warning('请选择统计字段')
    return
  }
  
  loading.value = true
  statsResult.value = null
  try {
    const timeRangeData = getTimeRange()
    let result
    if (isWeb) {
      result = await WebAPI.GetFieldStats({
        filterPolicyId: selectedPolicyId.value,
        startTime: timeRangeData.startTime,
        endTime: timeRangeData.endTime,
        field: selectedField.value,
        topN: topN.value
      })
    } else {
      result = await GetFieldStats({
        filterPolicyId: selectedPolicyId.value,
        startTime: timeRangeData.startTime,
        endTime: timeRangeData.endTime,
        field: selectedField.value,
        topN: topN.value
      })
    }
    if (result) {
      statsResult.value = result
    } else {
      statsResult.value = {
        field: selectedField.value,
        totalLogs: 0,
        uniqueCount: 0,
        items: []
      }
    }
  } catch (e: any) {
    console.error('Query error:', e)
    ElMessage.error('查询失败: ' + (e.message || e))
    statsResult.value = {
      field: selectedField.value,
      totalLogs: 0,
      uniqueCount: 0,
      items: []
    }
  } finally {
    loading.value = false
  }
}

function getBarWidth(count: number): string {
  if (maxCount.value === 0) return '0%'
  return (count / maxCount.value * 100) + '%'
}

function copyValues() {
  if (!statsResult.value || statsResult.value.items.length === 0) {
    ElMessage.warning('没有数据可复制')
    return
  }
  
  const values = statsResult.value.items.map(i => i.value).join('\n')
  navigator.clipboard.writeText(values).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

function exportCSV() {
  if (!statsResult.value || statsResult.value.items.length === 0) {
    ElMessage.warning('没有数据可导出')
    return
  }
  
  let csv = '值,次数,占比,最近出现时间\n'
  for (const item of statsResult.value.items) {
    csv += `${item.value},${item.count},${item.percent},${item.lastSeen}\n`
  }
  
  const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `stats_${selectedField.value}_${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('导出成功')
}

function getFieldDisplayName(fieldName: string): string {
  const field = availableFields.value.find(f => f.name === fieldName)
  if (field && field.displayName) {
    return `${field.displayName}（${fieldName}）`
  }
  return fieldName
}
</script>

<template>
  <div class="stats-view">
    <el-card shadow="hover" class="main-card">
      <template #header>
        <div class="card-header">
          <span>数据统计</span>
        </div>
      </template>
      
      <div class="filter-section">
        <el-form :inline="true" class="filter-form">
          <el-form-item label="筛选策略">
            <el-select v-model="selectedPolicyId" placeholder="选择筛选策略" style="width: 200px;">
              <el-option 
                v-for="policy in filterPolicies" 
                :key="policy.id" 
                :label="policy.name" 
                :value="policy.id"
              />
            </el-select>
          </el-form-item>
          
          <el-form-item label="时间范围">
            <el-select v-model="timeRange" style="width: 140px;">
              <el-option 
                v-for="opt in timeRangeOptions" 
                :key="opt.value" 
                :label="opt.label" 
                :value="opt.value"
              />
            </el-select>
          </el-form-item>
          
          <el-form-item v-if="timeRange === 'custom'" label="开始时间">
            <el-date-picker 
              v-model="customStartTime" 
              type="datetime" 
              placeholder="开始时间"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DD HH:mm:ss"
            />
          </el-form-item>
          
          <el-form-item v-if="timeRange === 'custom'" label="结束时间">
            <el-date-picker 
              v-model="customEndTime" 
              type="datetime" 
              placeholder="结束时间"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DD HH:mm:ss"
            />
          </el-form-item>
          
          <el-form-item label="统计字段">
            <el-select v-model="selectedField" placeholder="选择字段" style="width: 200px;">
              <el-option 
                v-for="field in availableFields" 
                :key="field.name" 
                :label="`${field.displayName}（${field.name}）`" 
                :value="field.name"
              />
            </el-select>
          </el-form-item>
          
          <el-form-item label="显示数量">
            <el-select v-model="topN" style="width: 100px;">
              <el-option 
                v-for="opt in topNOptions" 
                :key="opt.value" 
                :label="opt.label" 
                :value="opt.value"
              />
            </el-select>
          </el-form-item>
          
          <el-form-item>
            <el-button type="primary" :loading="loading" @click="handleQuery">
              <el-icon><Search /></el-icon>
              查询
            </el-button>
          </el-form-item>
        </el-form>
      </div>
      
      <div v-if="!selectedPolicyId" class="empty-state">
        <el-empty description="请先选择筛选策略" />
      </div>
      
      <div v-else-if="statsResult" class="stats-content">
        <div class="stats-summary">
          <el-row :gutter="20">
            <el-col :span="8">
              <div class="summary-item">
                <div class="summary-value">{{ statsResult.totalLogs }}</div>
                <div class="summary-label">总日志数</div>
              </div>
            </el-col>
            <el-col :span="8">
              <div class="summary-item">
                <div class="summary-value">{{ statsResult.uniqueCount }}</div>
                <div class="summary-label">唯一值数量</div>
              </div>
            </el-col>
            <el-col :span="8">
              <div class="summary-item">
                <div class="summary-value">{{ statsResult.items.length }}</div>
                <div class="summary-label">当前显示</div>
              </div>
            </el-col>
          </el-row>
        </div>
        
        <div v-if="statsResult.items.length > 0" class="chart-section">
          <div class="section-header">
            <span>Top {{ statsResult.items.length }} 分布图</span>
          </div>
          <div class="bar-chart">
            <div 
              v-for="(item, index) in statsResult.items" 
              :key="index" 
              class="bar-item"
            >
              <div class="bar-label">
                <span class="bar-rank">{{ index + 1 }}</span>
                <span class="bar-value">{{ item.value }}</span>
              </div>
              <div class="bar-container">
                <div class="bar-fill" :style="{ width: getBarWidth(item.count) }"></div>
              </div>
              <div class="bar-count">{{ item.count }}</div>
            </div>
          </div>
        </div>
        
        <div class="table-section">
          <div class="section-header">
            <span>详细数据</span>
            <div class="section-actions">
              <el-button size="small" @click="copyValues">
                <el-icon><CopyDocument /></el-icon>
                复制值
              </el-button>
              <el-button size="small" @click="exportCSV">
                <el-icon><Download /></el-icon>
                导出CSV
              </el-button>
            </div>
          </div>
          
          <el-table :data="statsResult.items" stripe>
            <el-table-column type="index" label="#" width="50" />
            <el-table-column prop="value" :label="getFieldDisplayName(selectedField)" min-width="150" />
            <el-table-column prop="count" label="次数" width="80" sortable />
            <el-table-column prop="percent" label="占比" width="70" />
            <el-table-column prop="lastSeen" label="最近出现" width="160" />
          </el-table>
        </div>
      </div>
      
      <div v-else class="empty-state">
        <el-empty description="点击查询按钮开始统计" />
      </div>
    </el-card>
  </div>
</template>

<style lang="scss" scoped>
.stats-view {
  .main-card {
    background: var(--bg-card);
    border-radius: 12px;
    border: 1px solid var(--border-color);
  }
  
  .card-header {
    font-size: 16px;
    font-weight: 600;
  }
  
  .filter-section {
    margin-bottom: 20px;
    padding-bottom: 20px;
    border-bottom: 1px solid var(--border-color);
    
    .filter-form {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      
      :deep(.el-form-item) {
        margin-bottom: 0;
      }
    }
  }
  
  .empty-state {
    padding: 60px 0;
    text-align: center;
  }
  
  .stats-content {
    .stats-summary {
      margin-bottom: 24px;
      
      .summary-item {
        text-align: center;
        padding: 16px;
        background: var(--bg-secondary);
        border-radius: 8px;
        
        .summary-value {
          font-size: 28px;
          font-weight: 600;
          color: var(--accent-color);
        }
        
        .summary-label {
          font-size: 13px;
          color: var(--text-secondary);
          margin-top: 4px;
        }
      }
    }
    
    .section-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      font-weight: 500;
      
      .section-actions {
        display: flex;
        gap: 8px;
      }
    }
    
    .chart-section {
      margin-bottom: 24px;
      
      .bar-chart {
        background: var(--bg-secondary);
        border-radius: 8px;
        padding: 16px;
        
        .bar-item {
          display: flex;
          align-items: center;
          margin-bottom: 12px;
          
          &:last-child {
            margin-bottom: 0;
          }
          
          .bar-label {
            width: 200px;
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 13px;
            
            .bar-rank {
              width: 20px;
              text-align: center;
              color: var(--text-secondary);
            }
            
            .bar-value {
              flex: 1;
              overflow: hidden;
              text-overflow: ellipsis;
              white-space: nowrap;
              font-family: monospace;
            }
          }
          
          .bar-container {
            flex: 1;
            height: 20px;
            background: var(--bg-hover);
            border-radius: 4px;
            overflow: hidden;
            
            .bar-fill {
              height: 100%;
              background: linear-gradient(90deg, var(--accent-color), var(--accent-color-light, var(--accent-color)));
              border-radius: 4px;
              transition: width 0.3s ease;
            }
          }
          
          .bar-count {
            width: 60px;
            text-align: right;
            font-size: 13px;
            color: var(--text-secondary);
          }
        }
      }
    }
    
    .table-section {
      :deep(.el-table) {
        background: transparent;
        
        th.el-table__cell {
          background: var(--bg-secondary);
        }
        
        tr {
          background: transparent;
          
          &:hover > td {
            background: var(--bg-hover);
          }
        }
      }
    }
  }
}
</style>
