<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="brand">
        <el-icon :size="26"><Cpu /></el-icon>
        <div>
          <div class="title">数据采集系统</div>
          <div class="subtitle">设备模板导入 · 按模板添加设备 · 采集运行</div>
        </div>
      </div>
      <div class="service-status" :class="{ bad: !healthOk }">
        <span class="dot"></span>
        <span>{{ healthOk ? `服务正常 · ${version || '-'}` : '服务不可用' }}</span>
      </div>
    </header>

    <main class="app-main">
      <el-tabs v-model="activeTab" class="main-tabs">
        <el-tab-pane label="设备模板" name="templates">
          <TemplateView />
        </el-tab-pane>
        <el-tab-pane label="设备管理" name="devices">
          <DeviceView />
        </el-tab-pane>
        <el-tab-pane label="采集数据" name="data">
          <DataView />
        </el-tab-pane>
        <el-tab-pane label="三方接口" name="subsystem">
          <SubsystemApiView />
        </el-tab-pane>
      </el-tabs>
    </main>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from './api'
import TemplateView from './views/TemplateView.vue'
import DeviceView from './views/DeviceView.vue'
import DataView from './views/DataView.vue'
import SubsystemApiView from './views/SubsystemApiView.vue'

const activeTab = ref('templates')
const healthOk = ref(false)
const version = ref('')
let timer = null

async function checkHealth() {
  try {
    const data = await api.health()
    healthOk.value = data?.status === 'ok'
    version.value = data?.version || ''
  } catch {
    healthOk.value = false
  }
}

onMounted(() => {
  checkHealth()
  timer = setInterval(checkHealth, 10000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>
