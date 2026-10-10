<template>
  <el-dialog
    v-model="visible"
    :title="mode === 'history' ? `历史数据：${device?.name || ''}` : `测点实时值：${device?.name || ''}`"
    width="920px"
    @open="onOpen"
    @closed="onClosed"
  >
    <!-- 实时值视图 -->
    <div v-if="mode === 'realtime'">
      <div class="toolbar">
        <el-tag :type="realtimeRunning ? 'success' : 'info'" effect="plain">
          {{ realtimeRunning ? '采集运行中' : '采集未运行' }}
        </el-tag>
        <div class="spacer"></div>
        <el-checkbox v-model="autoRefresh">自动刷新（3s）</el-checkbox>
        <el-button :icon="Refresh" size="small" @click="loadPoints">刷新</el-button>
      </div>
      <el-table :data="points" v-loading="loading" size="small" border max-height="420">
        <el-table-column prop="point_id" label="测点ID" width="80" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="当前值" min-width="140">
          <template #default="{ row }">
            <span>{{ formatValue(row.value) }}</span>
            <span v-if="row.unit" class="text-muted" style="margin-left: 4px">{{ row.unit }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="data_type" label="数据类型" width="110" />
        <el-table-column prop="access" label="读写" width="70" />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column prop="timestamp" label="更新时间" width="170" />
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="showHistory(row)">历史</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="暂无测点" /></template>
      </el-table>
    </div>

    <!-- 历史视图 -->
    <div v-else>
      <div class="toolbar">
        <el-button :icon="Back" size="small" @click="mode = 'realtime'">返回实时值</el-button>
        <el-tag effect="plain">测点 {{ historyPoint?.point_id }} · {{ historyPoint?.name }}</el-tag>
        <div class="spacer"></div>
        <el-button :icon="Refresh" size="small" @click="loadHistory">刷新</el-button>
      </div>
      <el-table :data="historyRows" v-loading="historyLoading" size="small" border max-height="420">
        <el-table-column prop="timestamp" label="时间" width="200" />
        <el-table-column label="数值" min-width="160">
          <template #default="{ row }">{{ formatValue(row.value) }}</template>
        </el-table-column>
        <el-table-column prop="value_kind" label="类型" width="120" />
        <template #empty><el-empty description="暂无历史数据（整点落盘，刚启动时可能为空）" /></template>
      </el-table>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { Back, Refresh } from '@element-plus/icons-vue'
import { api } from '../api'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  device: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const loading = ref(false)
const points = ref([])
const realtimeRunning = ref(false)
const autoRefresh = ref(true)
let timer = null

const mode = ref('realtime')
const historyPoint = ref(null)
const historyRows = ref([])
const historyLoading = ref(false)

function formatValue(value) {
  if (value === null || value === undefined || value === '') return '-'
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') return Number.isInteger(value) ? value : Number(value.toFixed(4))
  return String(value)
}

async function loadPoints() {
  if (!props.device?.id) return
  loading.value = true
  try {
    const res = await api.devicePoints(props.device.id)
    points.value = res?.data || []
    realtimeRunning.value = !!res?.running
  } catch {
    points.value = []
  } finally {
    loading.value = false
  }
}

async function showHistory(row) {
  historyPoint.value = row
  mode.value = 'history'
  await loadHistory()
}

async function loadHistory() {
  if (!props.device?.id || !historyPoint.value) return
  historyLoading.value = true
  try {
    const res = await api.deviceHistory(props.device.id, {
      point_id: historyPoint.value.point_id,
      limit: 200,
    })
    historyRows.value = res?.data || []
  } catch {
    historyRows.value = []
  } finally {
    historyLoading.value = false
  }
}

function startTimer() {
  stopTimer()
  timer = setInterval(() => {
    if (autoRefresh.value && mode.value === 'realtime' && visible.value) loadPoints()
  }, 3000)
}

function stopTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function onOpen() {
  mode.value = 'realtime'
  historyRows.value = []
  historyPoint.value = null
  loadPoints()
  startTimer()
}

function onClosed() {
  stopTimer()
}

onBeforeUnmount(stopTimer)
</script>
