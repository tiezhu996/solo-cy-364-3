<template>
  <el-table :data="alerts" v-loading="loading" border stripe>
    <el-table-column label="状态" width="90">
      <template #default="{ row }">
        <el-tag v-if="!row.is_read" type="danger" size="small">未读</el-tag>
        <el-tag v-else type="info" size="small">已读</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="门店" min-width="150">
      <template #default="{ row }">{{ row.store?.name || `门店#${row.store_id}` }}</template>
    </el-table-column>
    <el-table-column label="商品" min-width="180">
      <template #default="{ row }">
        <div>{{ row.sku?.name || `商品#${row.sku_id}` }}</div>
        <div class="sku-code">{{ row.sku?.code }}</div>
      </template>
    </el-table-column>
    <el-table-column label="当前库存" prop="quantity" width="100" align="right" />
    <el-table-column label="安全库存" prop="safety_stock" width="100" align="right" />
    <el-table-column label="缺口" width="100" align="right">
      <template #default="{ row }">
        <span class="shortage">-{{ row.shortage_qty }}</span>
      </template>
    </el-table-column>
    <el-table-column label="触发次数" prop="trigger_count" width="90" align="right" />
    <el-table-column label="触发时间" min-width="170">
      <template #default="{ row }">{{ formatDateTime(row.triggered_at) }}</template>
    </el-table-column>
    <el-table-column v-if="showAction" label="操作" width="110" fixed="right">
      <template #default="{ row }">
        <el-button v-if="!row.is_read" link type="primary" size="small" @click="emit('mark-read', row)">
          标记已读
        </el-button>
        <span v-else class="read-text">—</span>
      </template>
    </el-table-column>
    <template #empty>
      <el-empty :description="emptyText" />
    </template>
  </el-table>
</template>

<script setup lang="ts">
import type { StockAlert } from '@/types'
import { formatDateTime } from '@/utils/dateFormat'

defineProps<{
  alerts: StockAlert[]
  loading?: boolean
  showAction?: boolean
  emptyText?: string
}>()

const emit = defineEmits<{
  (e: 'mark-read', row: StockAlert): void
}>()
</script>

<style scoped>
.sku-code { color: #909399; font-size: 12px; }
.shortage { color: #f56c6c; font-weight: 600; }
.read-text { color: #c0c4cc; }
</style>
