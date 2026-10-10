<template>
  <el-dialog
    v-model="visible"
    :title="detail?.template?.name ? `模板详情：${detail.template.name}` : '模板详情'"
    width="920px"
    @open="load"
  >
    <div v-loading="loading">
      <el-descriptions v-if="detail" :column="3" border size="small" style="margin-bottom: 14px">
        <el-descriptions-item label="模板ID">{{ detail.template?.id }}</el-descriptions-item>
        <el-descriptions-item label="类型">{{ detail.template?.type }}</el-descriptions-item>
        <el-descriptions-item label="内置">
          {{ detail.template?.is_built_in ? '是' : '否' }}
        </el-descriptions-item>
        <el-descriptions-item label="描述" :span="3">
          {{ detail.template?.description || '-' }}
        </el-descriptions-item>
      </el-descriptions>

      <el-tabs v-model="tab">
        <el-tab-pane :label="`测点 (${detail?.points?.length || 0})`" name="points">
          <el-table :data="pointsShown" size="small" border max-height="360">
            <el-table-column prop="point_id" label="测点ID" width="80" />
            <el-table-column prop="name" label="名称" min-width="130" />
            <el-table-column prop="name_en" label="英文名" min-width="130" show-overflow-tooltip />
            <el-table-column prop="data_type" label="数据类型" width="110" />
            <el-table-column prop="unit" label="单位" width="80" />
            <el-table-column prop="access" label="读写" width="70" />
            <el-table-column label="地址/OID" min-width="130" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.oid || row.addr || (row.has_register_addr ? `寄存器 ${row.register_addr}` : '-') }}
              </template>
            </el-table-column>
            <el-table-column label="启用" width="70">
              <template #default="{ row }">{{ row.enabled ? '是' : '否' }}</template>
            </el-table-column>
            <el-table-column prop="description" label="描述" min-width="150" show-overflow-tooltip />
          </el-table>
          <div v-if="pointsTruncated" class="text-muted" style="margin-top: 8px">
            测点共 {{ detail?.points?.length || 0 }} 条，仅展示前 {{ DETAIL_LIMIT }} 条。
          </div>
        </el-tab-pane>

        <el-tab-pane :label="`告警 (${detail?.alarms?.length || 0})`" name="alarms">
          <el-table :data="alarmsShown" size="small" border max-height="360">
            <el-table-column prop="point_id" label="测点ID" width="80" />
            <el-table-column prop="name" label="名称" min-width="150" />
            <el-table-column prop="level" label="级别" width="90" />
            <el-table-column prop="start_high_limit" label="触发上限" width="100" />
            <el-table-column prop="start_low_limit" label="触发下限" width="100" />
            <el-table-column prop="end_high_limit" label="恢复上限" width="100" />
            <el-table-column prop="end_low_limit" label="恢复下限" width="100" />
            <el-table-column prop="description" label="描述" min-width="140" show-overflow-tooltip />
          </el-table>
          <div v-if="alarmsTruncated" class="text-muted" style="margin-top: 8px">
            告警共 {{ detail?.alarms?.length || 0 }} 条，仅展示前 {{ DETAIL_LIMIT }} 条。
          </div>
        </el-tab-pane>

        <el-tab-pane :label="`引用设备 (${detail?.references?.length || 0})`" name="references">
          <el-table :data="detail?.references || []" size="small" border max-height="360">
            <el-table-column prop="device_id" label="设备ID" width="100" />
            <el-table-column prop="name" label="设备名称" min-width="180" />
            <el-table-column prop="status" label="状态" width="120" />
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { api } from '../api'

// 大模板详情同样限制渲染行数，避免浏览器卡死。
const DETAIL_LIMIT = 200

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  templateId: { type: [Number, String], default: null },
})
const emit = defineEmits(['update:modelValue'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const loading = ref(false)
const detail = ref(null)
const tab = ref('points')

const pointsShown = computed(() => (detail.value?.points || []).slice(0, DETAIL_LIMIT))
const alarmsShown = computed(() => (detail.value?.alarms || []).slice(0, DETAIL_LIMIT))
const pointsTruncated = computed(() => (detail.value?.points || []).length > DETAIL_LIMIT)
const alarmsTruncated = computed(() => (detail.value?.alarms || []).length > DETAIL_LIMIT)

async function load() {
  if (!props.templateId) return
  loading.value = true
  try {
    const res = await api.templateDetail(props.templateId)
    detail.value = res?.data || null
  } catch {
    detail.value = null
  } finally {
    loading.value = false
  }
}

watch(() => props.templateId, () => {
  if (props.modelValue) load()
})
</script>
