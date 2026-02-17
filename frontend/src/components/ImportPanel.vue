<script setup lang="ts">
import { ref } from 'vue'
import { useNodeStore } from '../stores/nodes'

const store = useNodeStore()
const activeTab = ref<'file' | 'text' | 'sub'>('text')
const textInput = ref('')
const subUrls = ref('')
const importing = ref(false)

async function handleFileImport() {
  importing.value = true
  try {
    await store.importMultipleFiles()
    store.addLog('文件导入成功')
  } catch (err: any) {
    store.addLog(`导入失败: ${err}`)
  } finally {
    importing.value = false
  }
}

async function handleTextImport() {
  if (!textInput.value.trim()) return
  importing.value = true
  try {
    await store.importFromText(textInput.value)
    store.addLog('文本导入成功')
    textInput.value = ''
  } catch (err: any) {
    store.addLog(`导入失败: ${err}`)
  } finally {
    importing.value = false
  }
}

async function handleSubImport() {
  const urls = subUrls.value.split('\n').map(u => u.trim()).filter(u => u)
  if (urls.length === 0) return

  importing.value = true
  try {
    await store.importMultipleSubscriptions(urls)
    store.addLog(`成功导入 ${urls.length} 个订阅`)
    subUrls.value = ''
  } catch (err: any) {
    store.addLog(`订阅导入失败: ${err}`)
  } finally {
    importing.value = false
  }
}

async function handleClear() {
  await store.clearAllNodes()
  store.addLog('已清空所有节点')
}

async function handleDeleteSelected() {
  if (store.selectedCount === 0) {
    store.addLog('请先选择要删除的节点')
    return
  }
  const count = store.selectedCount
  await store.deleteSelectedNodes()
  store.addLog(`已删除 ${count} 个节点`)
}
</script>

<template>
  <div class="bg-gray-800 rounded-lg p-4">
    <h2 class="text-sm font-semibold text-gray-300 mb-3">导入节点</h2>

    <!-- Tabs -->
    <div class="flex gap-1 mb-3 bg-gray-900 rounded p-1">
      <button
        @click="activeTab = 'text'"
        :class="['flex-1 px-2 py-1.5 text-xs rounded transition-colors', activeTab === 'text' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200']"
      >
        粘贴文本
      </button>
      <button
        @click="activeTab = 'file'"
        :class="['flex-1 px-2 py-1.5 text-xs rounded transition-colors', activeTab === 'file' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200']"
      >
        选择文件
      </button>
      <button
        @click="activeTab = 'sub'"
        :class="['flex-1 px-2 py-1.5 text-xs rounded transition-colors', activeTab === 'sub' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200']"
      >
        订阅链接
      </button>
    </div>

    <!-- Text Input -->
    <div v-if="activeTab === 'text'" class="space-y-2">
      <textarea
        v-model="textInput"
        placeholder="粘贴节点链接或订阅内容...&#10;支持多行，每行一个节点"
        class="w-full h-32 bg-gray-900 border border-gray-700 rounded px-3 py-2 text-xs resize-none focus:outline-none focus:border-blue-500"
      />
      <button
        @click="handleTextImport"
        :disabled="importing || !textInput.trim()"
        class="w-full py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-xs font-medium transition-colors"
      >
        {{ importing ? '导入中...' : '导入' }}
      </button>
    </div>

    <!-- File Input -->
    <div v-else-if="activeTab === 'file'" class="space-y-2">
      <button
        @click="handleFileImport"
        :disabled="importing"
        class="w-full py-8 bg-gray-900 border-2 border-dashed border-gray-700 hover:border-gray-500 rounded text-xs text-gray-400 transition-colors"
      >
        {{ importing ? '导入中...' : '点击选择文件（可多选）' }}
      </button>
      <p class="text-gray-500 text-[10px] text-center">支持 .txt, .yaml, .yml, .conf 文件</p>
    </div>

    <!-- Subscription Input -->
    <div v-else class="space-y-2">
      <textarea
        v-model="subUrls"
        placeholder="输入订阅链接，每行一个...&#10;支持同时导入多个订阅"
        class="w-full h-32 bg-gray-900 border border-gray-700 rounded px-3 py-2 text-xs resize-none focus:outline-none focus:border-blue-500"
      />
      <button
        @click="handleSubImport"
        :disabled="importing || !subUrls.trim()"
        class="w-full py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-xs font-medium transition-colors"
      >
        {{ importing ? '获取中...' : '获取订阅' }}
      </button>
    </div>

    <!-- Action Buttons -->
    <div class="flex gap-2 mt-3 pt-3 border-t border-gray-700">
      <button
        @click="handleClear"
        :disabled="store.nodes.length === 0"
        class="flex-1 py-1.5 bg-red-600 hover:bg-red-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-xs font-medium transition-colors"
      >
        清空全部
      </button>
      <button
        @click="handleDeleteSelected"
        :disabled="store.selectedCount === 0"
        class="flex-1 py-1.5 bg-orange-600 hover:bg-orange-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-xs font-medium transition-colors"
      >
        删除选中 ({{ store.selectedCount }})
      </button>
    </div>
  </div>
</template>
