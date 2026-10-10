<template>
  <el-dialog
    v-model="visible"
    title="导入设备模板"
    width="880px"
    :close-on-click-modal="false"
    @open="onOpen"
    @closed="reset"
  >
    <el-steps :active="step" simple style="margin-bottom: 18px">
      <el-step title="上传模板文件" />
      <el-step title="预览并确认导入" />
    </el-steps>

    <!-- 第一步：选择类型 + 上传文件 -->
    <div v-show="step === 0">
      <el-form label-width="110px">
        <el-form-item label="模板类型" required>
          <el-select v-model="form.template_type" placeholder="请选择模板类型" style="width: 260px">
            <el-option
              v-for="t in templateTypes"
              :key="t"
              :label="typeLabel(t)"
              :value="t"
            />
          </el-select>
          <span class="text-muted" style="margin-left: 12px">
            类型决定模板的解析方式（Modbus / SNMP / 本地开关量）
          </span>
        </el-form-item>
        <el-form-item label="模板名称">
          <el-input
            v-model="form.template_name"
            placeholder="不填则使用模板文件中的名称"
            style="width: 320px"
          />
        </el-form-item>
        <el-form-item label="模板文件" required>
          <el-upload
            drag
            :auto-upload="false"
            :limit="1"
            accept=".xlsx,.xls"
            :on-change="onFileChange"
            :on-remove="onFileRemove"
            :file-list="fileList"
            style="width: 100%"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">将 xlsx 模板拖到此处，或<em>点击选择文件</em></div>
            <template #tip>
              <div class="el-upload__tip">
                支持 .xlsx / .xls；模板格式说明见仓库 examples/templates 与开发文档
              </div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
    </div>

    <!-- 第二步：预览 -->
    <div v-show="step === 1" v-loading="confirming">
      <el-descriptions :column="2" border size="small" style="margin-bottom: 14px">
        <el-descriptions-item label="模板名称">
          <el-input v-model="preview.template_name" size="small" style="width: 220px" />
        </el-descriptions-item>
        <el-descriptions-item label="模板类型">{{ preview.template_type }}</el-descriptions-item>
        <el-descriptions-item label="测点数量">{{ preview.points?.length || 0 }}</el-descriptions-item>
        <el-descriptions-item label="告警数量">{{ preview.alarms?.length || 0 }}</el-descriptions-item>
        <el-descriptions-item label="描述" :span="2">
          {{ preview.description || '-' }}
        </el-descriptions-item>
      </el-descriptions>

      <el-tabs v-model="previewTab">
        <el-tab-pane :label="`测点预览 (${preview.points?.length || 0})`" name="points">
          <el-table :data="previewPointsShown" size="small" border max-height="320">
            <el-table-column prop="point_id" label="测点ID" width="80" />
            <el-table-column prop="name" label="名称" min-width="140" />
            <el-table-column prop="data_type" label="数据类型" width="110" />
            <el-table-column prop="unit" label="单位" width="80" />
            <el-table-column prop="access" label="读写" width="80" />
            <el-table-column prop="addr" label="地址" width="110" show-overflow-tooltip />
            <el-table-column prop="oid" label="OID" min-width="140" show-overflow-tooltip />
            <el-table-column prop="description" label="描述" min-width="140" show-overflow-tooltip />
          </el-table>
          <div v-if="pointsTruncated" class="text-muted" style="margin-top: 8px">
            测点共 {{ preview.points?.length || 0 }} 条，预览仅展示前 {{ PREVIEW_LIMIT }} 条；确认导入时会完整保存。
          </div>
        </el-tab-pane>
        <el-tab-pane :label="`告警预览 (${preview.alarms?.length || 0})`" name="alarms">
          <el-table :data="previewAlarmsShown" size="small" border max-height="320">
            <el-table-column prop="point_id" label="测点ID" width="80" />
            <el-table-column prop="name" label="名称" min-width="150" />
            <el-table-column prop="level" label="级别" width="100" />
            <el-table-column label="启用" width="80">
              <template #default="{ row }">{{ row.enabled ? '是' : '否' }}</template>
            </el-table-column>
            <el-table-column prop="start_high_limit" label="触发上限" width="100" />
            <el-table-column prop="start_low_limit" label="触发下限" width="100" />
            <el-table-column prop="description" label="描述" min-width="150" show-overflow-tooltip />
          </el-table>
          <div class="text-muted" style="margin-top: 8px">
            说明：告警页签仅随模板保存为数据，本采集进程不做告警判定与通知。
            <span v-if="alarmsTruncated">（共 {{ preview.alarms?.length || 0 }} 条，仅展示前 {{ PREVIEW_LIMIT }} 条）</span>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button v-if="step === 1" @click="step = 0">上一步</el-button>
      <el-button v-if="step === 0" type="primary" :loading="parsing" @click="doPreview">
        解析预览
      </el-button>
      <el-button v-else type="primary" :loading="confirming" @click="doConfirm">
        确认导入
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import { api } from '../api'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'imported'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

// 大模板（数千测点）如果整表渲染会卡死浏览器，预览只渲染前 N 行。
const PREVIEW_LIMIT = 200

const templateTypes = ref([])
const step = ref(0)
const previewTab = ref('points')
const parsing = ref(false)
const confirming = ref(false)
const fileList = ref([])
const currentFile = ref(null)
const preview = ref({})

const form = ref({
  template_type: '',
  template_name: '',
})

const previewPointsShown = computed(() => (preview.value.points || []).slice(0, PREVIEW_LIMIT))
const previewAlarmsShown = computed(() => (preview.value.alarms || []).slice(0, PREVIEW_LIMIT))
const pointsTruncated = computed(() => (preview.value.points || []).length > PREVIEW_LIMIT)
const alarmsTruncated = computed(() => (preview.value.alarms || []).length > PREVIEW_LIMIT)

const typeLabels = {
  modbus: 'Modbus 模板',
  snmp: 'SNMP 模板',
  local_io: '本地开关量模板',
  local_di: '本地开关量输入模板',
  local_do: '本地开关量输出模板',
}

function typeLabel(t) {
  return typeLabels[t] || t
}

async function onOpen() {
  try {
    const res = await api.templateTypes()
    templateTypes.value = res?.data || []
    if (!form.value.template_type && templateTypes.value.length) {
      // 默认优先 Modbus（最常见的导入类型），否则取列表第一项。
      form.value.template_type = templateTypes.value.includes('modbus')
        ? 'modbus'
        : templateTypes.value[0]
    }
  } catch {
    /* 拦截器已提示 */
  }
}

function reset() {
  step.value = 0
  previewTab.value = 'points'
  fileList.value = []
  currentFile.value = null
  preview.value = {}
  form.value.template_name = ''
}

function onFileChange(file) {
  currentFile.value = file.raw
  fileList.value = [file]
}

function onFileRemove() {
  currentFile.value = null
  fileList.value = []
}

async function doPreview() {
  if (!form.value.template_type) {
    ElMessage.warning('请选择模板类型')
    return
  }
  if (!currentFile.value) {
    ElMessage.warning('请选择模板文件')
    return
  }
  const fd = new FormData()
  fd.append('file', currentFile.value)
  fd.append('template_type', form.value.template_type)
  if (form.value.template_name) fd.append('template_name', form.value.template_name)

  parsing.value = true
  try {
    const res = await api.templatePreview(fd)
    preview.value = res?.data || {}
    step.value = 1
  } catch {
    /* 拦截器已提示 */
  } finally {
    parsing.value = false
  }
}

async function doConfirm() {
  const payload = {
    template_name: (preview.value.template_name || '').trim(),
    description: preview.value.description || '',
    template_type: preview.value.template_type,
    points: preview.value.points || [],
    alarms: preview.value.alarms || [],
  }
  if (!payload.template_name) {
    ElMessage.warning('模板名称不能为空')
    return
  }
  confirming.value = true
  try {
    await api.templateConfirm(payload)
    ElMessage.success('模板导入成功')
    emit('imported')
    visible.value = false
  } catch {
    /* 拦截器已提示 */
  } finally {
    confirming.value = false
  }
}
</script>
