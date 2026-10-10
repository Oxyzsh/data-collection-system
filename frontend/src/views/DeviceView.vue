<template>
  <div class="panel">
    <div class="toolbar">
      <el-input
        v-model="keyword"
        placeholder="按名称 / 模板过滤"
        clearable
        style="width: 240px"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-tag type="info" effect="plain">共 {{ filtered.length }} 台设备</el-tag>
      <div class="spacer"></div>
      <el-checkbox v-model="autoRefresh">自动刷新（5s）</el-checkbox>
      <el-button :icon="Refresh" @click="load" :loading="loading">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="addVisible = true">添加设备</el-button>
    </div>

    <el-table :data="filtered" v-loading="loading" stripe border>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="设备名称" min-width="180" />
      <el-table-column prop="template_name" label="使用模板" min-width="160" />
      <el-table-column label="运行状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.status === 'running' ? 'success' : 'info'" effect="plain">
            {{ row.status === 'running' ? '运行中' : '已停止' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="100">
        <template #default="{ row }">
          <el-switch
            :model-value="!!row.enabled"
            :loading="togglingId === row.id"
            @change="(value) => toggleEnabled(row, value)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="io_channel" label="IO通道" width="100" />
      <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openPoints(row)">测点</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="暂无设备，点击右上角「添加设备」" />
      </template>
    </el-table>

    <AddDeviceDialog v-model="addVisible" @added="load" />
    <PointValuesDialog v-model="pointsVisible" :device="pointsDevice" />
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search } from '@element-plus/icons-vue'
import { api } from '../api'
import AddDeviceDialog from '../components/AddDeviceDialog.vue'
import PointValuesDialog from '../components/PointValuesDialog.vue'

const loading = ref(false)
const devices = ref([])
const keyword = ref('')
const autoRefresh = ref(true)
const addVisible = ref(false)
const togglingId = ref(null)
const pointsVisible = ref(false)
const pointsDevice = ref(null)
let timer = null

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return devices.value
  return devices.value.filter(
    (d) =>
      String(d.name || '').toLowerCase().includes(kw) ||
      String(d.template_name || '').toLowerCase().includes(kw),
  )
})

async function load() {
  loading.value = true
  try {
    const res = await api.devices()
    devices.value = res?.data || []
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false
  }
}

async function toggleEnabled(row, enabled) {
  togglingId.value = row.id
  try {
    await api.setDeviceEnabled(row.id, enabled)
    ElMessage.success(enabled ? '已启用并开始采集' : '已禁用')
  } catch {
    /* 启用失败（如串口打开失败）由拦截器提示 */
  } finally {
    togglingId.value = null
    load()
  }
}

function openPoints(row) {
  pointsDevice.value = row
  pointsVisible.value = true
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(
      `确认删除设备「${row.name}」？删除后历史数据仍会保留。`,
      '删除确认',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  try {
    await api.deleteDevice(row.id)
    ElMessage.success('删除成功')
    load()
  } catch {
    /* 拦截器已提示 */
  }
}

onMounted(() => {
  load()
  timer = setInterval(() => {
    if (autoRefresh.value) load()
  }, 5000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>
