<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useNodeStore } from './stores/nodes'
import NodeTable from './components/NodeTable.vue'
import ImportPanel from './components/ImportPanel.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import ToolBar from './components/ToolBar.vue'
import LogPanel from './components/LogPanel.vue'

const store = useNodeStore()

onMounted(() => {
  store.setupListeners()
})

onUnmounted(() => {
  store.teardownListeners()
})
</script>

<template>
  <div class="h-screen flex flex-col bg-gray-950 text-gray-100">
    <!-- Header -->
    <header class="bg-gray-900 border-b border-gray-800 px-4 py-3 flex items-center justify-between">
      <h1 class="text-xl font-semibold">节点延迟测试</h1>
      <div class="flex items-center gap-4 text-sm text-gray-400">
        <span>节点数: {{ store.nodes.length }}</span>
        <span>通过: {{ store.passingNodes }}</span>
        <span v-if="store.running" class="text-green-400 animate-pulse">测试中...</span>
        <span v-else-if="store.ipLookupProgress.total > 0" class="text-blue-400">
          查询出口IP: {{ store.ipLookupProgress.done }}/{{ store.ipLookupProgress.total }}
        </span>
      </div>
    </header>

    <!-- Main Content -->
    <div class="flex-1 flex overflow-hidden">
      <!-- Left Panel: Settings & Import -->
      <aside class="w-96 bg-gray-900 border-r border-gray-800 flex flex-col">
        <div class="flex-1 overflow-y-auto p-4 space-y-4">
          <ImportPanel />
          <SettingsPanel />
        </div>
        <LogPanel />
      </aside>

      <!-- Right Panel: Node Table -->
      <main class="flex-1 flex flex-col min-w-0">
        <ToolBar />
        <NodeTable />
      </main>
    </div>
  </div>
</template>
