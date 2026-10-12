<template>
  <div class="panel">
    <!-- 查询条件（设备选择移到左侧列表） -->
    <div class="toolbar">
      <el-select v-model="rangePreset" style="width: 140px" @change="onPresetChange">
        <el-option label="最近 1 小时" value="1h" />
        <el-option label="最近 24 小时" value="24h" />
        <el-option label="今天" value="today" />
        <el-option label="不限时间" value="all" />
      </el-select>

      <el-date-picker
        v-model="range"
        type="datetimerange"
        value-format="YYYY-MM-DD HH:mm:ss"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        style="width: 360px"
      />

      <el-select v-model="limit" style="width: 120px">
        <el-option :value="100" label="100 条" />
        <el-option :value="500" label="500 条" />
        <el-option :value="1000" label="1000 条" />
        <el-option :value="5000" label="5000 条" />
      </el-select>

      <div class="spacer"></div>
      <el-checkbox v-model="autoRefresh">自动刷新当前值（5s）</el-checkbox>
      <el-button :icon="Refresh" :loading="pointsLoading" @click="refreshAll">刷新</el-button>
    </div>

    <el-row :gutter="16">
      <!-- 左：设备列表（数量多时便于搜索与查看状态） -->
      <el-col :xs="24" :md="7">
        <el-card shadow="never" class="sub-card">
          <template #header>
            <div class="card-header">
              <span>设备</span>
              <el-tag size="small" effect="plain">共 {{ filteredDevices.length }} 台</el-tag>
            </div>
          </template>
          <el-input
            v-model="deviceKeyword"
            placeholder="搜索设备名称 / 模板"
            clearable
            size="small"
            style="margin-bottom: 8px"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-checkbox v-model="onlyRunning" size="small" style="margin-bottom: 8px">仅显示运行中</el-checkbox>

          <el-table
            :data="filteredDevices"
            v-loading="devicesLoading"
            size="small"
            highlight-current-row
            :current-row-key="deviceId"
            row-key="id"
            max-height="620"
            @current-change="onDeviceSelect"
          >
            <el-table-column prop="name" label="设备名称" min-width="130" show-overflow-tooltip />
            <el-table-column prop="template_name" label="模板" min-width="110" show-overflow-tooltip />
            <el-table-column label="状态" width="88">
              <template #default="{ row }">
                <el-tag size="small" :type="row.status === 'running' ? 'success' : 'info'" effect="plain">
                  {{ row.status === 'running' ? '运行中' : '已停止' }}
                </el-tag>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="没有匹配的设备" :image-size="60" />
            </template>
          </el-table>
        </el-card>
      </el-col>

      <!-- 右：测点当前值 + 历史数据 -->
      <el-col :xs="24" :md="17">
        <el-empty v-if="!deviceId" description="请在左侧选择设备" />

        <template v-else>
          <el-card shadow="never" class="sub-card">
            <template #header>
              <div class="card-header">
                <span>
                  测点（当前值）
                  <span class="text-muted" style="margin-left: 6px">{{ currentDeviceName }}</span>
                </span>
                <el-tag size="small" :type="running ? 'success' : 'info'" effect="plain">
                  {{ running ? '采集运行中' : '采集未运行' }}
                </el-tag>
              </div>
            </template>
            <el-table
              :data="points"
              v-loading="pointsLoading"
              size="small"
              border
              highlight-current-row
              :current-row-key="selectedPoint?.point_id"
              row-key="point_id"
              max-height="320"
              @current-change="onPointSelect"
            >
              <el-table-column prop="point_id" label="ID" width="56" />
              <el-table-column prop="name" label="测点名称" min-width="120" show-overflow-tooltip />
              <el-table-column label="当前值" min-width="120">
                <template #default="{ row }">
                  <span>{{ formatValue(row.value) }}</span>
                  <span v-if="row.unit" class="text-muted" style="margin-left: 4px">{{ row.unit }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="data_type" label="类型" width="86" />
              <el-table-column prop="access" label="读写" width="66" />
              <el-table-column prop="timestamp" label="更新时间" width="150" />
            </el-table>
          </el-card>

          <el-card shadow="never" class="sub-card">
            <template #header>
              <div class="card-header">
                <span>
                  历史数据
                  <template v-if="selectedPoint">
                    · {{ selectedPoint.name }}（测点 {{ selectedPoint.point_id }}）
                  </template>
                </span>
                <div>
                  <el-tag v-if="history.length" size="small" effect="plain">共 {{ history.length }} 条</el-tag>
                  <el-button
                    size="small"
                    :disabled="!history.length"
                    style="margin-left: 8px"
                    @click="exportCSV"
                  >
                    导出 CSV
                  </el-button>
                </div>
              </div>
            </template>

            <el-empty v-if="!selectedPoint" description="点击上方测点查看历史数据" />

            <template v-else>
              <div v-loading="historyLoading">
                <div v-if="chart" class="chart-wrap">
                  <svg :viewBox="`0 0 ${chart.width} ${chart.height}`" preserveAspectRatio="none">
                    <polyline :points="chart.polyline" fill="none" stroke="#2a6bb5" stroke-width="1.5" />
                    <text x="4" :y="18" class="chart-label">{{ chart.max }}</text>
                    <text x="4" :y="chart.height - 6" class="chart-label">{{ chart.min }}</text>
                  </svg>
                  <div class="chart-axis">
                    <span>{{ displayTime(history[0]?.timestamp) }}</span>
                    <span>{{ displayTime(history[history.length - 1]?.timestamp) }}</span>
                  </div>
                </div>

                <el-table :data="historyDesc" size="small" border max-height="360">
                  <el-table-column label="采集时间" width="180">
                    <template #default="{ row }">{{ displayTime(row.timestamp) }}</template>
                  </el-table-column>
                  <el-table-column label="数值" min-width="120">
                    <template #default="{ row }">{{ formatValue(row.value) }}</template>
                  </el-table-column>
                  <el-table-column prop="value_kind" label="类型" width="90" />
                  <template #empty>
                    <el-empty description="暂无历史数据（历史按整点错峰落盘，未到落盘窗口时为空）" />
                  </template>
                </el-table>
              </div>
            </template>
          </el-card>
        </template>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { api } from '../api'

const devices = ref([])
const devicesLoading = ref(false)
const deviceId = ref(null)
const deviceKeyword = ref('')
const onlyRunning = ref(false)

// 设备列表筛选：按名称/模板搜索，可选仅看运行中
const filteredDevices = computed(() => {
  const kw = deviceKeyword.value.trim().toLowerCase()
  return devices.value.filter((d) => {
    if (onlyRunning.value && d.status !== 'running') return false
    if (!kw) return true
    return (
      String(d.name || '').toLowerCase().includes(kw) ||
      String(d.template_name || '').toLowerCase().includes(kw)
    )
  })
})

const currentDeviceName = computed(() => {
  const d = devices.value.find((x) => x.id === deviceId.value)
  return d ? d.name : ''
})
const rangePreset = ref('24h')
const range = ref([])
const limit = ref(500)
const autoRefresh = ref(true)

const points = ref([])
const pointsLoading = ref(false)
const running = ref(false)
const selectedPoint = ref(null)

const history = ref([])
const historyLoading = ref(false)

let timer = null

const historyDesc = computed(() => [...history.value].reverse())

function pad(n) {
  return String(n).padStart(2, '0')
}

function fmtDateTime(d) {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function applyPreset(preset) {
  const now = new Date()
  if (preset === 'all') {
    range.value = []
    return
  }
  if (preset === 'today') {
    const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0)
    range.value = [fmtDateTime(start), fmtDateTime(now)]
    return
  }
  const hours = preset === '1h' ? 1 : 24
  const start = new Date(now.getTime() - hours * 3600 * 1000)
  range.value = [fmtDateTime(start), fmtDateTime(now)]
}

function onPresetChange(v) {
  applyPreset(v)
  if (selectedPoint.value) loadHistory()
}

// 历史时间可能是 "2026-10-12 12:05:39"（采集写入）或 RFC3339（部分驱动返回），统一展示。
function displayTime(t) {
  if (!t) return '-'
  return String(t).replace('T', ' ').replace(/Z$/, '')
}

function formatValue(value) {
  if (value === null || value === undefined || value === '') return '-'
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') return Number.isInteger(value) ? value : Number(value.toFixed(4))
  return String(value)
}

async function loadDevices() {
  devicesLoading.value = true
  try {
    const res = await api.devices()
    devices.value = res?.data || []
    if (!deviceId.value && devices.value.length) {
      deviceId.value = devices.value[0].id
      onDeviceChange()
    }
  } catch {
    /* 拦截器已提示 */
  } finally {
    devicesLoading.value = false
  }
}

// 点击左侧设备列表选中设备
function onDeviceSelect(row) {
  if (!row) return
  if (row.id === deviceId.value) return
  deviceId.value = row.id
  onDeviceChange()
}

function refreshAll() {
  loadDevices()
  if (deviceId.value) loadPoints()
}

async function loadPoints() {
  if (!deviceId.value) return
  pointsLoading.value = true
  try {
    const res = await api.devicePoints(deviceId.value)
    points.value = res?.data || []
    running.value = !!res?.running
    // 当前选中测点被刷新时同步最新元数据
    if (selectedPoint.value) {
      const fresh = points.value.find((p) => p.point_id === selectedPoint.value.point_id)
      if (fresh) selectedPoint.value = fresh
    }
  } catch {
    points.value = []
  } finally {
    pointsLoading.value = false
  }
}

async function loadHistory() {
  if (!deviceId.value || !selectedPoint.value) return
  historyLoading.value = true
  try {
    const params = {
      point_id: selectedPoint.value.point_id,
      limit: limit.value,
    }
    if (range.value?.length === 2) {
      params.start = range.value[0]
      params.end = range.value[1]
    }
    const res = await api.deviceHistory(deviceId.value, params)
    history.value = res?.data || []
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

function onDeviceChange() {
  selectedPoint.value = null
  history.value = []
  loadPoints()
}

function onPointSelect(row) {
  if (!row) return
  selectedPoint.value = row
  loadHistory()
}

// 简易 SVG 折线：仅对数值/布尔序列绘制，不引入图表库。
const chart = computed(() => {
  const rows = (history.value || []).filter((r) => typeof r.value === 'number')
  if (rows.length < 2) return null
  const values = rows.map((r) => Number(r.value))
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 1
  const width = 600
  const height = 160
  const padX = 12
  const padY = 24
  const points = values.map((v, i) => {
    const x = padX + (i * (width - padX * 2)) / (values.length - 1)
    const y = height - padY - ((v - min) / span) * (height - padY * 2)
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  return {
    width,
    height,
    min: Number(min.toFixed(3)),
    max: Number(max.toFixed(3)),
    polyline: points.join(' '),
  }
})

function exportCSV() {
  const rows = [['采集时间', '数值', '类型']]
  for (const r of history.value) {
    rows.push([displayTime(r.timestamp), formatValue(r.value), r.value_kind])
  }
  const csv = rows.map((r) => r.map((c) => `"${String(c ?? '').replace(/"/g, '""')}"`).join(',')).join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `device-${deviceId.value}-point-${selectedPoint.value?.point_id}-${Date.now()}.csv`
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('已导出 CSV')
}

onMounted(() => {
  applyPreset(rangePreset.value)
  loadDevices()
  timer = setInterval(() => {
    if (autoRefresh.value) loadPoints()
  }, 5000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.sub-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
}

.chart-wrap {
  margin-bottom: 12px;
}

.chart-wrap svg {
  width: 100%;
  height: 160px;
  background: #f7f9fc;
  border: 1px solid #ebeef5;
  border-radius: 4px;
}

.chart-label {
  font-size: 11px;
  fill: #909399;
}

.chart-axis {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>
