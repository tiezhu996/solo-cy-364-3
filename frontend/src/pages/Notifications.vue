<template>
  <div v-loading="store.loading">
    <el-card>
      <template #header>
        <div class="toolbar">
          <el-radio-group v-model="store.view" @change="onViewChange">
            <el-radio-button :value="AlertView.UNREAD">未读<el-badge v-if="store.unreadCount" :value="store.unreadCount" class="badge" /></el-radio-button>
            <el-radio-button :value="AlertView.ALL">全部</el-radio-button>
          </el-radio-group>
          <div>
            <el-button :icon="Refresh" @click="reload">刷新</el-button>
            <el-button type="primary" :icon="Check" :disabled="!hasUnread" @click="onMarkAll">全部已读</el-button>
          </div>
        </div>
      </template>

      <StockAlertTable
        :alerts="store.list"
        :loading="store.loading"
        show-action
        :empty-text="store.view === AlertView.UNREAD ? '暂无未读预警通知' : '暂无预警通知'"
        @mark-read="onMarkRead"
      />

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="store.total"
          :page-size="store.pageSize"
          :current-page="store.page"
          @current-change="onPageChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Check, Refresh } from '@element-plus/icons-vue'
import StockAlertTable from '@/components/common/StockAlertTable.vue'
import { useAlertStore } from '@/stores/alertStore'
import { AlertView } from '@/constants/stockAlert'
import type { StockAlert } from '@/types'

const store = useAlertStore()

const hasUnread = computed(() => store.list.some((a) => !a.is_read) || store.unreadCount > 0)

onMounted(load)

async function load() {
  await Promise.all([store.fetchList(), store.fetchUnreadCount()])
}

async function reload() {
  await load()
}

async function onViewChange() {
  await store.fetchList({ view: store.view, page: 1 })
}

async function onPageChange(page: number) {
  await store.fetchList({ page })
}

async function onMarkRead(row: StockAlert) {
  await store.markRead(row.id)
  ElMessage.success('已标记为已读')
  await load()
}

async function onMarkAll() {
  const affected = await store.markAllRead()
  ElMessage.success(affected > 0 ? `已将 ${affected} 条通知标记为已读` : '没有需要处理的未读通知')
  await load()
}
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; }
.badge { margin-left: 6px; }
.pager { margin-top: 16px; display: flex; justify-content: flex-end; }
</style>
