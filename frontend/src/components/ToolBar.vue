<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useNodeStore } from '../stores/nodes'

const store = useNodeStore()
const saving = ref(false)
const showExportMenu = ref(false)
const showTypeFilter = ref(false)
const selectedTypes = ref<Set<string>>(new Set())
const availableTypes = ref<string[]>([])
const exportMode = ref<'yaml' | 'yamlflow' | 'links'>('yaml')

const allProxyTypes = ['vless', 'vmess', 'trojan', 'ss', 'hysteria2', 'tuic', 'socks5', 'http']

async function handleStart() {
  if (store.nodes.length === 0) {
    store.addLog('请先导入节点')
    return
  }
  try {
    await store.startTest()
  } catch (err: any) {
    store.addLog(`启动失败: ${err}`)
  }
}

function handleStop() {
  store.stopTest()
}

async function handleSaveYAML() {
  if (store.passingNodes === 0) {
    store.addLog('没有通过的节点可导出')
    return
  }
  saving.value = true
  try {
    const yaml = await store.exportClashYAML()
    await navigator.clipboard.writeText(yaml)
    store.addLog('Clash 配置已复制到剪贴板')
  } catch (err: any) {
    store.addLog(`导出失败: ${err}`)
  } finally {
    saving.value = false
  }
}

async function handleSaveYAMLFlow() {
  if (store.passingNodes === 0) {
    store.addLog('没有通过的节点可导出')
    return
  }
  saving.value = true
  try {
    const yaml = await store.exportYAMLFlow()
    await navigator.clipboard.writeText(yaml)
    store.addLog('YAML Flow 格式已复制到剪贴板')
  } catch (err: any) {
    store.addLog(`导出失败: ${err}`)
  } finally {
    saving.value = false
    showExportMenu.value = false
  }
}

async function handleSaveLinks() {
  if (store.passingNodes === 0) {
    store.addLog('没有通过的节点可导出')
    return
  }
  try {
    const links = await store.exportNodeLinks()
    await navigator.clipboard.writeText(links)
    store.addLog('节点链接已复制到剪贴板')
  } catch (err: any) {
    store.addLog(`导出失败: ${err}`)
  }
}

function toggleType(type: string) {
  if (selectedTypes.value.has(type)) {
    selectedTypes.value.delete(type)
  } else {
    selectedTypes.value.add(type)
  }
}

function selectAllTypes() {
  allProxyTypes.forEach(t => selectedTypes.value.add(t))
}

function deselectAllTypes() {
  selectedTypes.value.clear()
}

async function doFilteredExport() {
  if (store.passingNodes === 0) {
    store.addLog('没有通过的节点可导出')
    return
  }
  if (selectedTypes.value.size === 0) {
    store.addLog('请至少选择一种代理类型')
    return
  }

  const typeFilter = Array.from(selectedTypes.value).join(',')
  saving.value = true

  try {
    let result: string
    if (exportMode.value === 'yamlflow') {
      result = await store.exportYAMLFlowFiltered(typeFilter)
      await navigator.clipboard.writeText(result)
      store.addLog(`YAML Flow 格式已复制到剪贴板 (${selectedTypes.value.size} 种类型)`)
    } else {
      result = await store.exportNodeLinksFiltered(typeFilter)
      await navigator.clipboard.writeText(result)
      store.addLog(`节点链接已复制到剪贴板 (${selectedTypes.value.size} 种类型)`)
    }
  } catch (err: any) {
    store.addLog(`导出失败: ${err}`)
  } finally {
    saving.value = false
    showTypeFilter.value = false
  }
}

function openTypeFilter(mode: 'yamlflow' | 'links') {
  exportMode.value = mode
  showTypeFilter.value = true
  showExportMenu.value = false
  loadAvailableTypes()
}

async function loadAvailableTypes() {
  try {
    const types = await store.getAvailableProxyTypes()
    availableTypes.value = types || []
    // 默认全选
    if (selectedTypes.value.size === 0) {
      types?.forEach(t => selectedTypes.value.add(t))
    }
  } catch (e) {
    availableTypes.value = allProxyTypes
  }
}

onMounted(() => {
  // 初始化默认全选
  allProxyTypes.forEach(t => selectedTypes.value.add(t))
})
</script>

<template>
  <div class="bg-gray-800 border-b border-gray-700 px-4 py-3 flex items-center justify-between">
    <!-- Progress -->
    <div class="flex items-center gap-4 text-sm">
      <div class="text-gray-400">
        进度: <span class="text-gray-200">{{ store.progress.done }}</span> / {{ store.progress.total }}
      </div>
      <div v-if="store.progress.total > 0" class="w-32 h-2 bg-gray-700 rounded-full overflow-hidden">
        <div
          class="h-full bg-blue-500 transition-all duration-300"
          :style="{ width: `${(store.progress.done / store.progress.total) * 100}%` }"
        />
      </div>
      <div class="text-green-400">
        通过: {{ store.progress.passed }}
      </div>
    </div>

    <!-- Buttons -->
    <div class="flex items-center gap-2">
      <button
        v-if="!store.running"
        @click="handleStart"
        :disabled="store.nodes.length === 0"
        class="px-4 py-2 bg-green-600 hover:bg-green-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-sm font-medium transition-colors"
      >
        开始测试
      </button>
      <button
        v-else
        @click="handleStop"
        class="px-4 py-2 bg-red-600 hover:bg-red-500 rounded text-sm font-medium transition-colors"
      >
        停止测试
      </button>

      <!-- Export Menu -->
      <div class="relative">
        <button
          @click="showExportMenu = !showExportMenu"
          :disabled="store.passingNodes === 0"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-sm font-medium transition-colors flex items-center gap-1"
        >
          导出
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <!-- Dropdown Menu -->
        <div
          v-if="showExportMenu"
          class="absolute right-0 mt-2 w-48 bg-gray-700 rounded-lg shadow-lg border border-gray-600 z-50"
        >
          <button
            @click="handleSaveYAML(); showExportMenu = false"
            class="w-full px-4 py-2 text-left text-sm hover:bg-gray-600 rounded-t-lg"
          >
            复制 YAML (完整)
          </button>
          <button
            @click="handleSaveYAMLFlow"
            class="w-full px-4 py-2 text-left text-sm hover:bg-gray-600"
          >
            复制 YAML Flow (全部类型)
          </button>
          <button
            @click="openTypeFilter('yamlflow')"
            class="w-full px-4 py-2 text-left text-sm hover:bg-gray-600"
          >
            复制 YAML Flow (筛选类型)
          </button>
          <button
            @click="handleSaveLinks(); showExportMenu = false"
            class="w-full px-4 py-2 text-left text-sm hover:bg-gray-600"
          >
            复制链接 (全部类型)
          </button>
          <button
            @click="openTypeFilter('links')"
            class="w-full px-4 py-2 text-left text-sm hover:bg-gray-600 rounded-b-lg"
          >
            复制链接 (筛选类型)
          </button>
        </div>
      </div>
    </div>

    <!-- Click outside to close menu -->
    <div
      v-if="showExportMenu"
      @click="showExportMenu = false"
      class="fixed inset-0 z-40"
    />

    <!-- Type Filter Modal -->
    <div
      v-if="showTypeFilter"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showTypeFilter = false"
    >
      <div class="bg-gray-800 rounded-lg p-6 w-96 border border-gray-700">
        <h3 class="text-lg font-medium mb-4">选择要导出的代理类型</h3>

        <div class="flex gap-2 mb-4">
          <button
            @click="selectAllTypes"
            class="px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 rounded"
          >
            全选
          </button>
          <button
            @click="deselectAllTypes"
            class="px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 rounded"
          >
            全不选
          </button>
        </div>

        <div class="grid grid-cols-2 gap-2 mb-4">
          <label
            v-for="type in allProxyTypes"
            :key="type"
            class="flex items-center gap-2 px-3 py-2 rounded cursor-pointer"
            :class="selectedTypes.has(type) ? 'bg-blue-600/30 border border-blue-500' : 'bg-gray-700 hover:bg-gray-600'"
          >
            <input
              type="checkbox"
              :checked="selectedTypes.has(type)"
              @change="toggleType(type)"
              class="rounded"
            />
            <span class="text-sm">{{ type }}</span>
          </label>
        </div>

        <div class="flex justify-end gap-2">
          <button
            @click="showTypeFilter = false"
            class="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded text-sm"
          >
            取消
          </button>
          <button
            @click="doFilteredExport"
            :disabled="saving || selectedTypes.size === 0"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-sm"
          >
            {{ saving ? '导出中...' : '导出' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
