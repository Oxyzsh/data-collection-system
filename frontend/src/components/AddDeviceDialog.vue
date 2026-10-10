<template>
  <el-dialog
    v-model="visible"
    title="添加设备（按模板）"
    width="760px"
    :close-on-click-modal="false"
    @open="onOpen"
  >
    <el-form label-width="120px" v-loading="loading">
      <el-form-item label="设备名称" required>
        <el-input v-model="form.name" placeholder="例如：机房温湿度-1" style="width: 320px" />
      </el-form-item>

      <el-form-item label="设备模板" required>
        <el-select
          v-model="form.template_id"
          placeholder="选择模板"
          style="width: 420px"
          filterable
        >
          <el-option
            v-for="t in templates"
            :key="t.id"
            :label="`${t.name}（${t.type}${t.is_built_in ? ' · 内置' : ''}）`"
            :value="t.id"
          />
        </el-select>
        <div v-if="selectedTemplate" class="text-muted" style="margin-left: 8px">
          {{ selectedTemplate.description || '按该模板实例化测点与参数' }}
        </div>
      </el-form-item>

      <!-- 协议设备：协议类型 -->
      <el-form-item v-if="templateKind === 'modbus' || templateKind === 'snmp'" label="协议类型" required>
        <el-select v-model="form.protocol_type" style="width: 260px" @change="onProtocolChange">
          <el-option
            v-for="p in availableProtocols"
            :key="p.type"
            :label="`${p.name_cn || p.type}（${p.type}）`"
            :value="p.type"
          />
        </el-select>
      </el-form-item>

      <!-- 串口设备：从 device.yaml 目录选择物理端点 -->
      <el-form-item v-if="needDeviceRef" :label="isLocal ? '本地 IO 控制器' : '串口端点'" required>
        <el-select
          v-model="form.device_ref"
          :placeholder="catalogLoading ? '加载中…' : '选择端点'"
          style="width: 360px"
          :loading="catalogLoading"
          @change="applyCatalogDefaults"
        >
          <el-option
            v-for="e in catalogEntries"
            :key="e.ref"
            :label="`${e.label} · ${e.summary}`"
            :value="e.ref"
          />
        </el-select>
        <span class="text-muted" style="margin-left: 8px">来自 device.yaml</span>
      </el-form-item>

      <!-- 本地 IO：方向 + 通道 -->
      <template v-if="isLocal">
        <el-form-item label="IO 方向" required>
          <el-radio-group v-model="localDirection" :disabled="templateType === 'local_di' || templateType === 'local_do'">
            <el-radio-button value="di">输入 DI</el-radio-button>
            <el-radio-button value="do">输出 DO</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="IO 通道" required>
          <el-select v-model="localChannel" placeholder="选择通道" style="width: 260px">
            <el-option
              v-for="c in localChannelOptions"
              :key="c.label"
              :label="`${c.label}（${c.direction === 'input' ? '输入' : '输出'}）`"
              :value="c.label"
            />
          </el-select>
        </el-form-item>
      </template>

      <!-- 协议参数（公共参数 + 协议专有参数） -->
      <template v-if="paramDefs.length">
        <el-divider content-position="left">采集参数</el-divider>
        <el-form-item
          v-for="p in paramDefs"
          :key="p.name"
          :label="p.name_cn || p.name"
          :required="p.required && !(needDeviceRef && p.name === 'serial')"
        >
          <!-- 串口路径由设备端点决定，不允许手填 -->
          <el-input
            v-if="needDeviceRef && p.name === 'serial'"
            :model-value="selectedCatalogSummary"
            disabled
            style="width: 320px"
          />

          <el-select
            v-else-if="p.type === 'enum'"
            v-model="paramValues[p.name]"
            style="width: 260px"
          >
            <el-option v-for="opt in p.options || []" :key="opt" :label="opt" :value="opt" />
          </el-select>

          <el-switch v-else-if="p.type === 'bool'" v-model="paramValues[p.name]" />

          <el-input-number
            v-else-if="p.type === 'int'"
            v-model="paramValues[p.name]"
            :min="p.min ?? undefined"
            :max="p.max ?? undefined"
            style="width: 260px"
          />

          <el-input
            v-else
            v-model="paramValues[p.name]"
            style="width: 320px"
            :placeholder="p.default || p.description || ''"
          />

          <span class="text-muted" style="margin-left: 10px">{{ p.description }}</span>
        </el-form-item>
      </template>

      <el-form-item label="描述">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="2"
          style="width: 520px"
          placeholder="可选"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">添加设备</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'added'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const loading = ref(false)
const submitting = ref(false)
const catalogLoading = ref(false)

const templates = ref([])
const protocolTypes = ref([])
const catalogEntries = ref([])
const localChannels = ref([])

const form = ref({
  name: '',
  template_id: null,
  protocol_type: '',
  device_ref: '',
  description: '',
})
const paramDefs = ref([])
const paramValues = ref({})
const localDirection = ref('di')
const localChannel = ref('')

const selectedTemplate = computed(() =>
  templates.value.find((t) => t.id === form.value.template_id),
)
const templateType = computed(() => (selectedTemplate.value?.type || '').toLowerCase())

const templateKind = computed(() => {
  const t = templateType.value
  if (t === 'modbus') return 'modbus'
  if (t === 'snmp') return 'snmp'
  if (t === 'local_io' || t === 'local_di' || t === 'local_do') return 'local'
  return 'plain'
})

const isLocal = computed(() => templateKind.value === 'local')
const isSerial = computed(() =>
  ['RS485-RTU', 'RS232-RTU'].includes(form.value.protocol_type),
)
const needDeviceRef = computed(() => isSerial.value || isLocal.value)

const availableProtocols = computed(() => {
  if (templateKind.value === 'snmp') {
    return protocolTypes.value.filter((p) =>
      String(p.type).toUpperCase().startsWith('SNMP'),
    )
  }
  return protocolTypes.value.filter((p) => String(p.category).toLowerCase() === 'modbus')
})

const catalogSelector = computed(() => {
  if (isLocal.value) return 'local_io'
  if (form.value.protocol_type === 'RS485-RTU') return 'rs485_rtu'
  if (form.value.protocol_type === 'RS232-RTU') return 'rs232_rtu'
  return ''
})

const selectedCatalogSummary = computed(() => {
  const e = catalogEntries.value.find((x) => x.ref === form.value.device_ref)
  return e?.summary || ''
})

const localChannelOptions = computed(() => {
  const want = localDirection.value === 'di' ? 'input' : 'output'
  return localChannels.value.filter((c) => c.direction === want)
})

async function onOpen() {
  loading.value = true
  reset()
  try {
    const [tplRes, protoRes] = await Promise.all([api.templates(), api.protocolTypes()])
    templates.value = tplRes?.data || []
    protocolTypes.value = protoRes?.data?.types || []
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false
  }
}

function reset() {
  form.value = { name: '', template_id: null, protocol_type: '', device_ref: '', description: '' }
  paramDefs.value = []
  paramValues.value = {}
  catalogEntries.value = []
  localChannels.value = []
  localDirection.value = 'di'
  localChannel.value = ''
}

async function onTemplateChange() {
  form.value.device_ref = ''
  paramDefs.value = []
  paramValues.value = {}
  catalogEntries.value = []
  localChannels.value = []
  localChannel.value = ''

  const kind = templateKind.value
  if (kind === 'modbus') {
    form.value.protocol_type = 'RS485-RTU'
    await onProtocolChange()
  } else if (kind === 'snmp') {
    form.value.protocol_type = 'SNMP'
    await onProtocolChange()
  } else if (kind === 'local') {
    form.value.protocol_type = templateType.value
    localDirection.value = templateType.value === 'local_do' ? 'do' : 'di'
    await loadCatalog()
  } else {
    form.value.protocol_type = ''
  }
}

async function onProtocolChange() {
  await loadProtocolParams()
  await loadCatalog()
}

async function loadProtocolParams() {
  if (!form.value.protocol_type) return
  try {
    const res = await api.protocolParams(form.value.protocol_type)
    const data = res?.data || {}
    paramDefs.value = [...(data.common_params || []), ...(data.protocol_params || [])]
    const values = {}
    for (const p of paramDefs.value) {
      values[p.name] = p.type === 'int' && p.default !== '' ? Number(p.default) : (p.default ?? '')
    }
    paramValues.value = values
    applyCatalogDefaults()
  } catch {
    /* 拦截器已提示 */
  }
}

async function loadCatalog() {
  const selector = catalogSelector.value
  catalogEntries.value = []
  if (!selector) return
  catalogLoading.value = true
  try {
    const res = await api.deviceCatalog(selector)
    catalogEntries.value = res?.data?.array || []
    if (isLocal.value && catalogEntries.value.length) {
      localChannels.value = catalogEntries.value[0].channels || []
      form.value.device_ref = catalogEntries.value[0].ref
    }
    if (catalogEntries.value.length === 1) {
      form.value.device_ref = catalogEntries.value[0].ref
    }
    applyCatalogDefaults()
  } catch {
    /* 拦截器已提示 */
  } finally {
    catalogLoading.value = false
  }
}

function applyCatalogDefaults() {
  const entry = catalogEntries.value.find((x) => x.ref === form.value.device_ref)
  if (!entry?.defaults) return
  const next = { ...paramValues.value }
  for (const [key, value] of Object.entries(entry.defaults)) {
    if (key in next) next[key] = value
  }
  paramValues.value = next
}

async function submit() {
  if (!form.value.name.trim()) return ElMessage.warning('请填写设备名称')
  if (!form.value.template_id) return ElMessage.warning('请选择设备模板')
  if (templateKind.value !== 'plain' && !form.value.protocol_type) {
    return ElMessage.warning('请选择协议类型')
  }
  if (needDeviceRef.value && !form.value.device_ref) {
    return ElMessage.warning(isLocal.value ? '请选择本地 IO 控制器' : '请选择串口端点')
  }
  if (isLocal.value && !localChannel.value) return ElMessage.warning('请选择 IO 通道')

  const params = {}
  for (const p of paramDefs.value) {
    if (needDeviceRef.value && p.name === 'serial') continue // 由设备端点固定
    const value = paramValues.value[p.name]
    if (value === undefined || value === null || String(value) === '') {
      if (p.required) return ElMessage.warning(`参数「${p.name_cn || p.name}」必填`)
      continue
    }
    params[p.name] = String(value)
  }
  if (isLocal.value) {
    params.io_channel = localChannel.value
    params.io_direction = localDirection.value
  }

  submitting.value = true
  try {
    await api.addDevice({
      name: form.value.name.trim(),
      template_id: form.value.template_id,
      protocol_type: form.value.protocol_type || undefined,
      device_ref: needDeviceRef.value ? form.value.device_ref : undefined,
      description: form.value.description,
      params,
    })
    ElMessage.success('设备添加成功')
    emit('added')
    visible.value = false
  } catch {
    /* 拦截器已提示（如参数校验失败） */
  } finally {
    submitting.value = false
  }
}

// 模板切换时重新装载该模板对应的协议与目录
watch(() => form.value.template_id, (val, old) => {
  if (val && val !== old) onTemplateChange()
})
</script>
