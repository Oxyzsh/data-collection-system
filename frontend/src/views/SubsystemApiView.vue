<template>
  <div class="panel">
    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="本系统作为「第三方系统」对外提供的子系统接口"
      description="供外部 Sub-System 拉取设备树与采集数据、下发控制命令；pointStr 约定为 <deviceId>.<pointId>（例如 1.2）。"
    />

    <!-- 接口清单 -->
    <el-table :data="endpoints" size="small" border style="margin-top: 14px">
      <el-table-column prop="method" label="方法" width="80">
        <template #default="{ row }">
          <el-tag size="small" :type="row.method === 'GET' ? 'success' : 'warning'" effect="plain">
            {{ row.method }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="path" label="路径" min-width="330" class-name="mono" />
      <el-table-column prop="desc" label="说明" min-width="260" />
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="row.call()">调用</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-row :gutter="16" style="margin-top: 16px">
      <!-- 设备树 -->
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="sub-card">
          <template #header>
            <div class="card-header">
              <span>设备树（点击测点填入 pointStr）</span>
              <el-button size="small" :loading="loadingTree" @click="loadTree">加载</el-button>
            </div>
          </template>
          <el-tree
            :data="treeData"
            :props="{ label: 'label', children: 'children' }"
            node-key="key"
            default-expand-all
            @node-click="onTreeNodeClick"
          />
          <el-empty v-if="!treeData.length" description="点击右上角「加载」拉取设备树" />
        </el-card>
      </el-col>

      <!-- 在线调用 -->
      <el-col :xs="24" :md="12">
        <el-card shadow="never" class="sub-card">
          <template #header>
            <div class="card-header"><span>在线调用</span></div>
          </template>

          <el-form label-width="92px" size="small">
            <el-form-item label="pointStr">
              <el-input v-model="pointStr" placeholder="例如 1.2" style="width: 200px" />
            </el-form-item>
            <el-form-item label="value">
              <el-input v-model="value" placeholder="控制点写入值，例如 28" style="width: 200px" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="calling" @click="callRealtime">查询实时值</el-button>
              <el-button type="warning" :loading="calling" @click="callSetValue">下发控制</el-button>
            </el-form-item>
          </el-form>

          <div class="text-muted" style="margin-bottom: 6px">响应（原始 JSON）</div>
          <pre class="resp">{{ responseText }}</pre>
        </el-card>
      </el-col>
    </el-row>

    <!-- curl 示例 -->
    <el-card shadow="never" class="sub-card" style="margin-top: 16px">
      <template #header>
        <div class="card-header">
          <span>curl 示例</span>
          <el-button size="small" @click="copyCurl">复制</el-button>
        </div>
      </template>
      <pre class="resp">{{ curlText }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { subsystemApi } from '../api'

const loadingTree = ref(false)
const calling = ref(false)
const pointStr = ref('1.1')
const value = ref('28')
const responseText = ref('（点击上方「调用」或「查询实时值」查看响应）')
const tree = ref([])

const origin = computed(() => (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8090'))

const endpoints = computed(() => [
  { method: 'GET', path: '/health', desc: '健康检查', call: callHealth },
  {
    method: 'GET',
    path: '/protocol/sub/system/api/v1/converter/deviceTree',
    desc: '设备树：设备为根节点，测点为叶子节点',
    call: callDeviceTree,
  },
  {
    method: 'GET',
    path: '/protocol/sub/system/api/v1/converter/realtime?pointStr={pointStr}',
    desc: '测点实时值（来自运行态）',
    call: callRealtime,
  },
  {
    method: 'POST',
    path: '/api/set/{pointStr}',
    desc: '设置测点值（仅 rw/w 测点可写）',
    call: callSetValue,
  },
])

const treeData = computed(() =>
  (tree.value || []).map((device) => ({
    key: device.id,
    label: `${device.name}（${device.id}）`,
    children: (device.children || []).map((point) => ({
      key: point.id,
      label: `${point.pointId} ${point.name}${point.unit ? ' (' + point.unit + ')' : ''}`,
      pointId: point.pointId,
      isLeaf: true,
    })),
  })),
)

const curlText = computed(
  () => `# 健康检查
curl ${origin.value}/health

# 设备树
curl ${origin.value}/protocol/sub/system/api/v1/converter/deviceTree

# 测点实时值
curl "${origin.value}/protocol/sub/system/api/v1/converter/realtime?pointStr=1.1"

# 设置测点值（控制）
curl -X POST ${origin.value}/api/set/1.3 \\
     -H 'Content-Type: application/json' \\
     -d '{"pointId":"1.3","value":28}'`,
)

function show(data) {
  responseText.value = JSON.stringify(data, null, 2)
}

async function callHealth() {
  calling.value = true
  try {
    show(await subsystemApi.health())
  } catch (e) {
    show({ error: String(e) })
  } finally {
    calling.value = false
  }
}

async function loadTree() {
  loadingTree.value = true
  try {
    const res = await subsystemApi.deviceTree()
    if (res.errorCode !== 0) {
      ElMessage.error(res.errorMsg || '获取设备树失败')
      return
    }
    tree.value = res.data || []
    ElMessage.success(`设备树已加载：${tree.value.length} 台设备`)
  } catch (e) {
    ElMessage.error('获取设备树失败：' + e)
  } finally {
    loadingTree.value = false
  }
}

async function callDeviceTree() {
  await loadTree()
  try {
    show(await subsystemApi.deviceTree())
  } catch (e) {
    show({ error: String(e) })
  }
}

async function callRealtime() {
  if (!pointStr.value.trim()) {
    ElMessage.warning('请填写 pointStr')
    return
  }
  calling.value = true
  try {
    show(await subsystemApi.realtime(pointStr.value.trim()))
  } catch (e) {
    show({ error: String(e) })
  } finally {
    calling.value = false
  }
}

async function callSetValue() {
  if (!pointStr.value.trim()) {
    ElMessage.warning('请填写 pointStr')
    return
  }
  calling.value = true
  try {
    const num = Number(value.value)
    show(await subsystemApi.setValue(pointStr.value.trim(), Number.isNaN(num) || value.value === '' ? value.value : num))
  } catch (e) {
    show({ error: String(e) })
  } finally {
    calling.value = false
  }
}

function onTreeNodeClick(node) {
  if (node?.isLeaf && node.pointId) {
    pointStr.value = node.pointId
    ElMessage.success(`已填入 pointStr=${node.pointId}`)
  }
}

async function copyCurl() {
  try {
    await navigator.clipboard.writeText(curlText.value)
    ElMessage.success('已复制 curl 示例')
  } catch {
    ElMessage.warning('浏览器不允许自动复制，请手动选择复制')
  }
}

onMounted(() => {
  callHealth()
})
</script>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
}

.resp {
  margin: 0;
  padding: 10px 12px;
  background: #f7f9fc;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 320px;
  overflow: auto;
}
</style>
