<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { SendTestSyslog, GetLocalIPs, GetConfig, GetLogTraceInfo } from '../../wailsjs/go/main/App'
import { WebAPI } from '../api/web'

const isWeb = typeof window !== 'undefined' && !(window as any).go

interface TestResult {
  success: boolean
  message: string
  sentCount: number
  failedCount: number
  errors: string[]
}

interface AlertTraceInfo {
  robotId: number
  robotName: string
  platform: string
  status: string
  errorMsg: string
  sentAt: string
}

interface LogTraceInfo {
  logId: number
  receivedAt: string
  sourceIp: string
  rawMessage: string
  receiveStatus: string
  receiveError: string
  parseStatus: string
  parseTemplate: string
  parsedData: string
  parseError: string
  filterStatus: string
  filterEnabled: boolean
  matchedPolicy: string
  alertStatus: string
  alertRecords: AlertTraceInfo[]
}

const loading = ref(false)
const sending = ref(false)
const localIPs = ref<string[]>([])
const listenPort = ref(5140)
const protocol = ref('udp')

const traceLogId = ref<number | null>(null)
const traceInfo = ref<LogTraceInfo | null>(null)
const traceLoading = ref(false)
const traceExpanded = ref(false)

const protocols = [
  { value: 'udp', label: 'UDP' },
  { value: 'tcp', label: 'TCP' }
]

const testForm = ref({
  host: '127.0.0.1',
  port: 5140,
  message: '',
  count: 1,
  intervalMs: 1000
})

const testResult = ref<TestResult | null>(null)

const sampleTemplates = [
  {
    name: '云锁 - 攻击成功',
    message: `<134>Mar 15 10:30:00 server01 {"event_type":"attack_success","attack_ip":"192.168.1.100","attack_type":"暴力破解","target_user":"admin","level":3,"description":"SSH暴力破解成功"}`
  },
  {
    name: '云锁 - 高危告警',
    message: `<134>Mar 15 10:30:00 server01 {"event_type":"high_risk","attack_ip":"10.0.0.50","threat_name":"WebShell检测","file_path":"/var/www/html/shell.php","level":4,"description":"发现WebShell后门"}`
  },
  {
    name: '云锁 - 异常登录',
    message: `<134>Mar 15 10:30:00 server01 {"event_type":"abnormal_login","login_ip":"203.0.113.50","login_user":"root","login_time":"2024-03-15 10:30:00","location":"美国","level":3,"description":"异地异常登录"}`
  },
  {
    name: '通用 - JSON格式',
    message: `{"timestamp":"2024-03-15T10:30:00Z","level":"error","source":"firewall","src_ip":"192.168.1.100","dst_ip":"10.0.0.1","action":"blocked","message":"可疑连接被阻止"}`
  },
  {
    name: '通用 - 键值对',
    message: `time=2024-03-15T10:30:00 src_ip=192.168.1.100 dst_ip=10.0.0.1 action=block reason="可疑连接" level=3`
  }
]

onMounted(async () => {
  await loadData()
})

async function loadData() {
  loading.value = true
  try {
    const [ips, config] = await Promise.all([
      GetLocalIPs(),
      GetConfig()
    ])
    localIPs.value = ips
    if (config && config.listenPort) {
      listenPort.value = config.listenPort
      testForm.value.port = config.listenPort
    }
    if (config && config.protocol) {
      protocol.value = config.protocol
    }
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

function applySample(sample: typeof sampleTemplates[0]) {
  testForm.value.message = sample.message
}

async function handleSend() {
  if (!testForm.value.message.trim()) {
    ElMessage.warning('请输入测试日志内容')
    return
  }

  sending.value = true
  testResult.value = null

  try {
    let result
    if (isWeb) {
      result = await WebAPI.SendTestSyslog({
        host: testForm.value.host,
        port: testForm.value.port,
        protocol: protocol.value,
        message: testForm.value.message,
        count: testForm.value.count,
        intervalMs: testForm.value.intervalMs
      })
    } else {
      result = await SendTestSyslog({
        host: testForm.value.host,
        port: testForm.value.port,
        protocol: protocol.value,
        message: testForm.value.message,
        count: testForm.value.count,
        intervalMs: testForm.value.intervalMs
      })
    }
    testResult.value = result

    if (result.success) {
      ElMessage.success(result.message)
    } else {
      ElMessage.warning(result.message)
    }
  } catch (e: any) {
    ElMessage.error('发送失败: ' + (e.message || e))
  } finally {
    sending.value = false
  }
}

function clearResult() {
  testResult.value = null
}

async function handleTraceLog() {
  if (!traceLogId.value) {
    ElMessage.warning('请输入日志ID')
    return
  }

  traceLoading.value = true
  traceInfo.value = null

  try {
    let info
    if (isWeb) {
      info = await WebAPI.GetLogTrace(traceLogId.value)
    } else {
      info = await GetLogTraceInfo(traceLogId.value)
    }
    traceInfo.value = info
    if (!info) {
      ElMessage.warning('未找到该日志ID的追踪信息')
    }
  } catch (e: any) {
    ElMessage.error('追踪失败: ' + (e.message || e))
  } finally {
    traceLoading.value = false
  }
}

function getNodeClass(status: string): string {
  if (status === 'success' || status === 'matched' || status === 'executing') return 'success'
  if (status === 'failed') return 'error'
  if (status === 'unmatched' || status === 'disabled' || status === 'pending' || status === 'none') return 'warning'
  return 'warning'
}

function getAlertNodeClass(): string {
  if (!traceInfo.value) return 'warning'
  const status = traceInfo.value.alertStatus
  if (status === 'sent' || status === 'executing') return 'success'
  if (status === 'failed') return 'error'
  if (status === 'pending' || status === 'none') return 'warning'
  return 'warning'
}

function getArrowClass(stage: string): string {
  if (!traceInfo.value) return 'warning'

  if (stage === 'receive') {
    return traceInfo.value.receiveStatus === 'success' ? 'success' : 'error'
  }

  if (stage === 'parse') {
    if (traceInfo.value.receiveStatus !== 'success') return 'warning'
    return traceInfo.value.parseStatus === 'success' ? 'success' : 'error'
  }

  if (stage === 'filter') {
    if (traceInfo.value.parseStatus !== 'success') return 'warning'
    if (traceInfo.value.filterStatus === 'matched') return 'success'
    if (traceInfo.value.filterStatus === 'disabled') return 'warning'
    if (traceInfo.value.filterStatus === 'unmatched') return 'warning'
    return 'warning'
  }

  if (stage === 'alert') {
    if (traceInfo.value.filterStatus === 'disabled') return 'warning'
    if (traceInfo.value.filterStatus !== 'matched') return 'warning'
    const status = traceInfo.value.alertStatus
    if (status === 'sent' || status === 'executing') return 'success'
    if (status === 'failed') return 'error'
    if (status === 'pending' || status === 'none') return 'warning'
    return 'warning'
  }

  if (stage === 'result') {
    if (traceInfo.value.filterStatus === 'disabled') return 'warning'
    if (traceInfo.value.filterStatus !== 'matched') return 'warning'
    const status = traceInfo.value.alertStatus
    if (status === 'sent' || status === 'executing') return 'success'
    if (status === 'failed') return 'error'
    if (status === 'pending' || status === 'none') return 'warning'
    return 'warning'
  }

  return 'warning'
}

function getResultNodeClass(): string {
  if (!traceInfo.value) return 'warning'

  if (traceInfo.value.receiveStatus === 'success' &&
      traceInfo.value.parseStatus === 'success' &&
      traceInfo.value.filterStatus === 'matched' &&
      (traceInfo.value.alertStatus === 'sent' || traceInfo.value.alertStatus === 'executing')) {
    return 'success'
  }

  if (traceInfo.value.receiveStatus === 'failed' ||
      traceInfo.value.parseStatus === 'failed' ||
      traceInfo.value.alertStatus === 'failed') {
    return 'error'
  }

  return 'warning'
}

function getResultIcon(): string {
  if (!traceInfo.value) return '○'

  if (traceInfo.value.receiveStatus === 'success' &&
      traceInfo.value.parseStatus === 'success' &&
      traceInfo.value.filterStatus === 'matched' &&
      (traceInfo.value.alertStatus === 'sent' || traceInfo.value.alertStatus === 'executing')) {
    return '✓'
  }

  if (traceInfo.value.receiveStatus === 'failed' ||
      traceInfo.value.parseStatus === 'failed' ||
      traceInfo.value.alertStatus === 'failed') {
    return '✗'
  }

  return '○'
}

function getNodeIcon(status: string): string {
  if (status === 'success' || status === 'matched' || status === 'executing') return '✓'
  if (status === 'failed') return '✗'
  return '○'
}
</script>

<template>
  <div class="test-tools-view">
    <div class="cards-container">
      <el-card shadow="hover" class="config-card">
        <template #header>
          <div class="card-header">
            <span>发送配置</span>
          </div>
        </template>

        <el-form :model="testForm" label-width="70px" class="send-form">
            <el-form-item label="目标地址">
              <div class="address-inputs">
                <el-input v-model="testForm.host" placeholder="默认 127.0.0.1" style="width: 200px;" />
                <span class="input-label">端口</span>
                <el-input v-model="testForm.port" placeholder="5140" style="width: 120px;" />
              </div>
              <div class="form-tip">
                本机IP:
                <el-tag v-for="ip in localIPs" :key="ip" size="small" style="margin-right: 5px; cursor: pointer;" @click="testForm.host = ip">
                  {{ ip }}
                </el-tag>
              </div>
            </el-form-item>

            <el-form-item label="测试日志">
              <el-input
                v-model="testForm.message"
                type="textarea"
                :rows="6"
                placeholder="输入要发送的 Syslog 测试数据..."
              />
            </el-form-item>

            <el-form-item label="发送次数">
              <el-input-number v-model="testForm.count" :min="1" :max="1000" />
              <span class="form-tip">连续发送多少条日志</span>
            </el-form-item>

            <el-form-item label="发送间隔">
              <el-input-number v-model="testForm.intervalMs" :min="0" :max="60000" :step="100" />
              <span class="form-tip">每条日志之间的间隔时间（毫秒）</span>
            </el-form-item>

            <el-form-item>
              <el-button type="primary" :loading="sending" @click="handleSend">
                <el-icon><Position /></el-icon>
                发送测试数据
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="hover" class="tips-card">
          <template #header>
            <span>使用说明</span>
          </template>

          <div class="tips-content">
            <p><strong>1. 配置目标地址</strong></p>
            <p>默认发送到本机的 Syslog 服务端口，也可以发送到其他主机。</p>

            <p><strong>2. 输入测试日志</strong></p>
            <p>手动输入要发送的 Syslog 测试数据。</p>

            <p><strong>3. 设置发送参数</strong></p>
            <p>发送次数：连续发送多少条相同的日志</p>
            <p>发送间隔：每条日志之间的时间间隔</p>

            <p><strong>4. 查看结果</strong></p>
            <p>发送完成后，可在「日志查看」页面查看接收到的日志，验证解析规则是否正确。</p>

            <el-alert
              type="info"
              :closable="false"
              style="margin-top: 15px;"
            >
              <template #title>
                提示：确保 Syslog 服务已启动
              </template>
            </el-alert>
          </div>
        </el-card>
      </div>

      <el-card shadow="hover" class="debug-card" style="margin-top: 20px;">
        <template #header>
          <div class="card-header">
            <span>日志ID追踪</span>
            <el-switch v-model="traceExpanded" active-text="展开" inactive-text="折叠" />
          </div>
        </template>

        <div v-show="traceExpanded" class="debug-content">
          <div class="trace-input">
            <el-input-number
              v-model="traceLogId"
              :min="1"
              placeholder="输入日志ID"
              style="width: 200px;"
            />
            <el-button type="primary" @click="handleTraceLog" :loading="traceLoading">
              追踪
            </el-button>
          </div>

          <div v-if="traceInfo" class="trace-flow">
                <div class="flow-title">处理流程</div>
                <div class="flow-container">
                  <div class="flow-line">
                    <div class="flow-node" :class="traceInfo.receiveStatus === 'success' ? 'success' : 'error'">
                      <div class="node-title">接收</div>
                      <div class="node-status">{{ traceInfo.receiveStatus === 'success' ? '✓' : '✗' }}</div>
                    </div>
                    <div class="flow-arrow" :class="getArrowClass('receive')"></div>
                    <div class="flow-node" :class="getNodeClass(traceInfo.parseStatus)">
                      <div class="node-title">解析</div>
                      <div class="node-status">{{ getNodeIcon(traceInfo.parseStatus) }}</div>
                    </div>
                    <div class="flow-arrow" :class="getArrowClass('parse')"></div>
                    <div class="flow-node" :class="getNodeClass(traceInfo.filterStatus)">
                      <div class="node-title">筛选</div>
                      <div class="node-status">{{ getNodeIcon(traceInfo.filterStatus) }}</div>
                    </div>
                    <div class="flow-arrow" :class="getArrowClass('filter')"></div>
                    <div class="flow-node" :class="getAlertNodeClass()">
                      <div class="node-title">推送</div>
                      <div class="node-status">{{ getNodeIcon(traceInfo.alertStatus) }}</div>
                    </div>
                    <div class="flow-arrow" :class="getArrowClass('result')"></div>
                    <div class="flow-node" :class="getResultNodeClass()">
                      <div class="node-title">结果</div>
                      <div class="node-status">{{ getResultIcon() }}</div>
                    </div>
                  </div>
                </div>

                <el-collapse style="margin-top: 20px;">
                  <el-collapse-item title="详细信息" name="details">
                    <el-descriptions :column="3" border size="small">
                      <el-descriptions-item label="日志ID">{{ traceInfo.logId }}</el-descriptions-item>
                      <el-descriptions-item label="接收时间">{{ traceInfo.receivedAt }}</el-descriptions-item>
                      <el-descriptions-item label="来源IP">{{ traceInfo.sourceIp || '-' }}</el-descriptions-item>
                      <el-descriptions-item label="解析模板">{{ traceInfo.parseTemplate || '-' }}</el-descriptions-item>
                      <el-descriptions-item label="筛选结果" :span="2">
                        <el-tag v-if="traceInfo.filterStatus === 'matched'" type="success" size="small">
                          已匹配策略: {{ traceInfo.matchedPolicy || '-' }}
                        </el-tag>
                        <el-tag v-else-if="traceInfo.filterStatus === 'disabled'" type="info" size="small">
                          筛选策略未启用
                        </el-tag>
                        <el-tag v-else-if="traceInfo.filterStatus === 'unmatched'" type="warning" size="small">
                          未匹配策略
                        </el-tag>
                        <span v-else>-</span>
                      </el-descriptions-item>
                      <el-descriptions-item label="原始消息" :span="3">
                        <el-input type="textarea" v-model="traceInfo.rawMessage" :rows="2" size="small" readonly />
                      </el-descriptions-item>
                      <el-descriptions-item label="推送状态" :span="3">
                        <template v-if="traceInfo.alertRecords && traceInfo.alertRecords.length > 0">
                          <el-tag v-for="record in traceInfo.alertRecords" :key="record.robotId" size="small" style="margin-right: 5px;">
                            {{ record.robotName }} ({{ record.platform }})
                          </el-tag>
                        </template>
                        <template v-else-if="traceInfo.filterStatus === 'matched' && traceInfo.alertStatus === 'none'">
                          <el-tag type="warning" size="small">未触发推送（策略未配置机器人或动作为丢弃）</el-tag>
                        </template>
                        <template v-else-if="traceInfo.filterStatus === 'unmatched'">
                          <el-tag type="info" size="small">未匹配策略，未进入推送流程</el-tag>
                        </template>
                        <span v-else>
                          <el-tag type="info" size="small">推送节点未启用</el-tag>
                        </span>
                      </el-descriptions-item>
                      <el-descriptions-item label="推送结果" :span="3">
                        <template v-if="traceInfo.alertRecords && traceInfo.alertRecords.length > 0">
                          <el-tag v-for="record in traceInfo.alertRecords" :key="record.robotId"
                            :type="record.status === 'sent' ? 'success' : 'danger'" size="small" style="margin-right: 10px;">
                            {{ record.robotName }}: {{ record.status === 'sent' ? '成功' : '失败' }}
                            <span v-if="record.errorMsg" style="color: #c0c4cc;"> - {{ record.errorMsg }}</span>
                          </el-tag>
                        </template>
                        <template v-else-if="traceInfo.filterStatus === 'matched' && traceInfo.alertStatus === 'none'">
                          <el-tag type="warning" size="small">未触发推送</el-tag>
                        </template>
                        <template v-else-if="traceInfo.filterStatus === 'unmatched'">
                          <el-tag type="info" size="small">未匹配策略</el-tag>
                        </template>
                        <template v-else>
                          <el-tag type="info" size="small">推送节点未启用</el-tag>
                        </template>
                      </el-descriptions-item>
                      <el-descriptions-item v-if="traceInfo.parseError" label="解析错误" :span="3">
                        <span style="color: #f56c6c;">{{ traceInfo.parseError }}</span>
                      </el-descriptions-item>
                    </el-descriptions>
                  </el-collapse-item>
                </el-collapse>
              </div>

              <el-empty v-if="!traceInfo && !traceLoading" description="输入日志ID进行追踪" />
            </div>
          </el-card>
  </div>
</template>

<style lang="scss" scoped>
.test-tools-view {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .form-tip {
    margin-left: 10px;
    color: var(--text-secondary);
    font-size: 12px;
  }

  .send-form {
    :deep(.el-form-item) {
      margin-bottom: 18px;
    }
  }

  .address-inputs {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: nowrap;
    white-space: nowrap;

    .input-label {
      color: var(--text-secondary);
      font-size: 14px;
      flex-shrink: 0;
    }
  }

  .cards-container {
    display: flex;
    gap: 20px;
    align-items: stretch;

    :deep(.el-card) {
      flex: 1;
      min-height: 350px;
      display: flex;
      flex-direction: column;
    }

    .tips-card {
      margin-top: 0;
    }
  }

  .debug-card {
    :deep(.el-card__body) {
      padding: 0;
    }

    .debug-content {
      padding: 15px 20px 0;
      transition: all 0.3s ease;
      overflow: hidden;

      &.is-collapsed {
        .debug-placeholder {
          display: flex;
        }
      }

      &:not(.is-collapsed) {
        .debug-placeholder {
          display: none;
        }
      }
    }

    .debug-placeholder {
      display: none;
      align-items: center;
      justify-content: center;
      gap: 10px;
      height: 80px;
      color: var(--text-muted);
      font-size: 14px;

      .placeholder-icon {
        font-size: 20px;
      }
    }

    .debug-body {
      padding: 20px;
      animation: slideDown 0.3s ease;

      :deep(.el-tabs) {
        height: 100%;
      }

      :deep(.el-tab-pane) {
        height: 100%;
        overflow: auto;
      }

      :deep(.el-empty) {
        padding: 20px 0;
      }
    }
  }

  @keyframes slideDown {
    from {
      opacity: 0;
      transform: translateY(-10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .trace-flow {
    margin-top: 15px;
    max-height: 250px;
    overflow-y: auto;
  }

  .result-card {
    :deep(.el-card__body) {
      padding: 12px 16px;
    }

    .result-compact {
      align-items: center;
      gap: 16px;

      .result-status {
        display: flex;
        align-items: center;
        gap: 6px;

        .status-text {
          font-size: 14px;
          font-weight: 500;

          &.success {
            color: #67c23a;
          }

          &.warning {
            color: #e6a23c;
          }
        }
      }

      .result-stats {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-left: auto;

        .stat-item {
          display: flex;
          align-items: center;
          gap: 4px;

          .stat-label {
            font-size: 12px;
            color: var(--el-text-color-secondary);
          }

          .stat-value {
            font-size: 14px;
            font-weight: 600;

            &.success {
              color: #67c23a;
            }

            &.error {
              color: #f56c6c;
            }
          }
        }

        .stat-divider {
          color: var(--el-border-color);
        }
      }
    }

    .error-list-compact {
      margin-top: 10px;
      padding-top: 10px;
      border-top: 1px solid var(--el-border-color-lighter);

      .error-item {
        font-size: 12px;
        color: #f56c6c;
        padding: 4px 0;
      }
    }
  }

  .sample-list {
    .sample-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 12px;
      margin-bottom: 8px;
      background: var(--bg-secondary);
      border-radius: 8px;
      cursor: pointer;
      transition: all 0.2s;

      &:hover {
        background: var(--accent-color);
        color: white;
      }

      &:last-child {
        margin-bottom: 0;
      }
    }
  }

  .tips-card {
    .tips-content {
      p {
        margin: 8px 0;
        color: var(--text-secondary);
        font-size: 13px;
        line-height: 1.6;

        &:first-child {
          margin-top: 0;
        }

        strong {
          color: var(--text-primary);
        }
      }
    }
  }

  .service-status {
    padding: 10px 0;
  }

  .trace-input {
    display: flex;
    gap: 10px;
    margin-bottom: 15px;
  }

  .trace-flow {
    margin-top: 15px;
  }

  .flow-title {
    font-size: 14px;
    font-weight: 500;
    margin-bottom: 15px;
    color: var(--text-primary);
  }

  .flow-container {
    background: var(--bg-secondary);
    border-radius: 8px;
    padding: 20px;
    overflow-x: auto;
  }

  .flow-line {
    display: flex;
    align-items: center;
    justify-content: center;
    min-width: max-content;
  }

  .flow-node {
    width: 60px;
    height: 60px;
    border-radius: 50%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    border: 3px solid;
    flex-shrink: 0;

    &.success {
      background: #f0f9ff;
      border-color: #67c23a;
      color: #67c23a;
    }

    &.error {
      background: #fef0f0;
      border-color: #f56c6c;
      color: #f56c6c;
    }

    &.warning {
      background: #fdf6ec;
      border-color: #e6a23c;
      color: #e6a23c;
    }

    .node-title {
      font-size: 12px;
      font-weight: 500;
    }

    .node-status {
      font-size: 14px;
      margin-top: 2px;
    }
  }

  .flow-arrow {
    width: 40px;
    height: 3px;
    margin: 0 8px;
    flex-shrink: 0;

    &.success {
      background: #67c23a;
    }

    &.error {
      background: #f56c6c;
    }

    &.warning {
      background: #e6a23c;
    }
  }
}
</style>
