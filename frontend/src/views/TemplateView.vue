<template>
  <div class="panel">
    <div class="toolbar">
      <el-input
        v-model="keyword"
        placeholder="按名称 / 类型过滤"
        clearable
        style="width: 240px"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-tag type="info" effect="plain">共 {{ filtered.length }} 个模板</el-tag>
      <div class="spacer"></div>
      <el-button :icon="Refresh" @click="load" :loading="loading">刷新</el-button>
      <el-button type="primary" :icon="Upload" @click="importVisible = true">
        导入设备模板
      </el-button>
    </div>

    <el-table :data="filtered" v-loading="loading" stripe border>
      <el-table-column prop="id" label="ID" width="90" />
      <el-table-column prop="name" label="模板名称" min-width="200">
        <template #default="{ row }">
          <span>{{ row.name }}</span>
          <el-tag v-if="row.is_built_in" size="small" type="warning" effect="plain" style="margin-left: 8px">
            内置
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="type" label="模板类型" width="140">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="描述" min-width="220" show-overflow-tooltip />
      <el-table-column prop="updated_at" label="更新时间" width="170" />
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">查看</el-button>
          <el-button
            link
            type="danger"
            :disabled="row.is_built_in"
            @click="remove(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="暂无模板，点击右上角「导入设备模板」" />
      </template>
    </el-table>

    <TemplateImportDialog v-model="importVisible" @imported="load" />
    <TemplateDetailDialog v-model="detailVisible" :template-id="detailId" />
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search, Upload } from '@element-plus/icons-vue'
import { api } from '../api'
import TemplateImportDialog from '../components/TemplateImportDialog.vue'
import TemplateDetailDialog from '../components/TemplateDetailDialog.vue'

const loading = ref(false)
const templates = ref([])
const keyword = ref('')
const importVisible = ref(false)
const detailVisible = ref(false)
const detailId = ref(null)

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return templates.value
  return templates.value.filter(
    (t) =>
      String(t.name || '').toLowerCase().includes(kw) ||
      String(t.type || '').toLowerCase().includes(kw),
  )
})

async function load() {
  loading.value = true
  try {
    const res = await api.templates()
    templates.value = res?.data || []
  } catch {
    /* 错误提示由拦截器统一处理 */
  } finally {
    loading.value = false
  }
}

function openDetail(row) {
  detailId.value = row.id
  detailVisible.value = true
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确认删除模板「${row.name}」？`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await api.templateDelete(row.id)
    ElMessage.success('删除成功')
    load()
  } catch {
    /* 模板被设备引用等错误由拦截器提示 */
  }
}

onMounted(load)
</script>
