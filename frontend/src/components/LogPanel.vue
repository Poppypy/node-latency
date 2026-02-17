<script setup lang="ts">
import { ref } from 'vue'
import { useNodeStore } from '../stores/nodes'

const store = useNodeStore()
const collapsed = ref(false)

function clearLogs() {
  store.logs = []
}
</script>

<template>
  <div class="bg-gray-850 border-t border-gray-700 flex flex-col" :class="collapsed ? 'h-8' : 'h-40'">
    <div class="flex items-center justify-between px-4 py-1.5 bg-gray-800 border-b border-gray-700 cursor-pointer" @click="collapsed = !collapsed">
      <span class="text-xs font-medium text-gray-400">日志</span>
      <div class="flex items-center gap-2">
        <button
          v-if="!collapsed"
          @click.stop="clearLogs"
          class="text-xs text-gray-500 hover:text-gray-300"
        >
          清空
        </button>
        <svg class="w-4 h-4 text-gray-400 transition-transform" :class="collapsed ? '' : 'rotate-180'" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </div>
    </div>
    <div v-show="!collapsed" class="flex-1 overflow-auto p-2 font-mono text-xs">
      <div v-for="(log, i) in store.logs" :key="i" class="text-gray-400 py-0.5">
        {{ log }}
      </div>
      <div v-if="store.logs.length === 0" class="text-gray-600 italic">
        暂无日志...
      </div>
    </div>
  </div>
</template>
