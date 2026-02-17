import { defineStore } from 'pinia'
import { ref, computed, reactive } from 'vue'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import {
  ImportFromText,
  ImportFromFile,
  ImportFromSubscription,
  ImportMultipleSubscriptions,
  ImportMultipleFiles,
  ClearNodes,
  DeleteNodes,
  StartTest,
  StopTest,
  ExportClashYAML,
  ExportNodeLinks,
  ExportYAMLFlow,
  ExportYAMLFlowFiltered,
  ExportNodeLinksFiltered,
  GetAvailableProxyTypes,
  UpdateSettings,
  GetSettings,
} from '../../wailsjs/go/main/App'
import { model } from '../../wailsjs/go/models'

export const useNodeStore = defineStore('nodes', () => {
  // State
  const nodes = ref<model.NodeDTO[]>([])
  // 使用 reactive 对象而不是 Map，确保响应式更新
  const results = reactive<Record<number, any>>({})
  const running = ref(false)
  const progress = ref({ total: 0, done: 0, passed: 0, running: false })
  const ipLookupProgress = ref({ done: 0, total: 0 })
  const logs = ref<string[]>([])
  const settings = ref<model.TestSettings>(new model.TestSettings())
  const selectedIndices = ref<Set<number>>(new Set())

  // Getters
  const passingNodes = computed(() => {
    return Object.values(results).filter((r: any) => r && r.pass).length
  })

  const selectedCount = computed(() => selectedIndices.value.size)

  // Actions
  function setupListeners() {
    EventsOn('test-result', (event: any) => {
      if (event && typeof event.index === 'number') {
        results[event.index] = event
      }
    })
    EventsOn('test-progress', (event: any) => {
      progress.value = event
    })
    EventsOn('test-complete', () => {
      running.value = false
      progress.value.running = false
    })
    EventsOn('log', (msg: string) => {
      addLog(msg)
    })
    EventsOn('nodes-updated', (updatedNodes: any) => {
      if (Array.isArray(updatedNodes)) {
        nodes.value = updatedNodes
      }
    })
    EventsOn('ip-lookup-progress', (event: any) => {
      ipLookupProgress.value = { done: event.done || 0, total: event.total || 0 }

      // 出口 IP 查询结束后，同步关闭运行态，避免 UI 一直停留在“测试中”
      if ((event.done || 0) >= (event.total || 0) && (event.total || 0) > 0) {
        running.value = false
        progress.value.running = false
      }
    })
  }

  function teardownListeners() {
    EventsOff('test-result')
    EventsOff('test-progress')
    EventsOff('test-complete')
    EventsOff('log')
    EventsOff('nodes-updated')
    EventsOff('ip-lookup-progress')
  }

  function addLog(msg: string) {
    const timestamp = new Date().toLocaleTimeString()
    logs.value.push(`[${timestamp}] ${msg}`)
    if (logs.value.length > 500) {
      logs.value.shift()
    }
  }

  function getResult(index: number) {
    return results[index]
  }

  function clearResults() {
    Object.keys(results).forEach(key => {
      delete results[parseInt(key)]
    })
  }

  function toggleSelect(index: number) {
    if (selectedIndices.value.has(index)) {
      selectedIndices.value.delete(index)
    } else {
      selectedIndices.value.add(index)
    }
  }

  function selectAll() {
    nodes.value.forEach((_, i) => selectedIndices.value.add(i))
  }

  function deselectAll() {
    selectedIndices.value.clear()
  }

  function selectFailed() {
    selectedIndices.value.clear()
    nodes.value.forEach((_, i) => {
      const r = results[i]
      if (r && r.done && !r.pass) {
        selectedIndices.value.add(i)
      }
    })
  }

  function selectPassed() {
    selectedIndices.value.clear()
    nodes.value.forEach((_, i) => {
      const r = results[i]
      if (r && r.done && r.pass) {
        selectedIndices.value.add(i)
      }
    })
  }

  async function importFromText(text: string): Promise<void> {
    const imported = await ImportFromText(text)
    nodes.value = imported || []
    clearResults()
    selectedIndices.value.clear()
    progress.value = { total: nodes.value.length, done: 0, passed: 0, running: false }
  }

  async function importFromFile(): Promise<void> {
    const imported = await ImportFromFile()
    nodes.value = imported || []
    clearResults()
    selectedIndices.value.clear()
    progress.value = { total: nodes.value.length, done: 0, passed: 0, running: false }
  }

  async function importFromSubscription(url: string): Promise<void> {
    const imported = await ImportFromSubscription(url)
    nodes.value = imported || []
    clearResults()
    selectedIndices.value.clear()
    progress.value = { total: nodes.value.length, done: 0, passed: 0, running: false }
  }

  async function importMultipleSubscriptions(urls: string[]): Promise<void> {
    const imported = await ImportMultipleSubscriptions(urls)
    nodes.value = imported || []
    clearResults()
    selectedIndices.value.clear()
    progress.value = { total: nodes.value.length, done: 0, passed: 0, running: false }
  }

  async function importMultipleFiles(): Promise<void> {
    const imported = await ImportMultipleFiles()
    nodes.value = imported || []
    clearResults()
    selectedIndices.value.clear()
    progress.value = { total: nodes.value.length, done: 0, passed: 0, running: false }
  }

  async function startTest(): Promise<void> {
    running.value = true
    progress.value.running = true
    clearResults()
    ipLookupProgress.value = { done: 0, total: 0 }
    await StartTest()
  }

  async function stopTest(): Promise<void> {
    await StopTest()
    running.value = false
  }

  async function updateSettings(newSettings: model.TestSettings): Promise<void> {
    settings.value = newSettings
    await UpdateSettings(newSettings)
  }

  async function loadSettings(): Promise<void> {
    const s = await GetSettings()
    settings.value = s
  }

  async function exportClashYAML(): Promise<string> {
    return await ExportClashYAML()
  }

  async function exportNodeLinks(): Promise<string> {
    const result = await ExportNodeLinks()
    return result || ''
  }

  async function exportYAMLFlow(): Promise<string> {
    const result = await ExportYAMLFlow()
    return result || ''
  }

  async function exportYAMLFlowFiltered(typeFilter: string): Promise<string> {
    const result = await ExportYAMLFlowFiltered(typeFilter)
    return result || ''
  }

  async function exportNodeLinksFiltered(typeFilter: string): Promise<string> {
    const result = await ExportNodeLinksFiltered(typeFilter)
    return result || ''
  }

  async function getAvailableProxyTypes(): Promise<string[]> {
    return await GetAvailableProxyTypes()
  }

  async function clearAllNodes(): Promise<void> {
    await ClearNodes()
    nodes.value = []
    clearResults()
    selectedIndices.value.clear()
    progress.value = { total: 0, done: 0, passed: 0, running: false }
  }

  async function deleteSelectedNodes(): Promise<void> {
    if (selectedIndices.value.size === 0) return
    const indices = Array.from(selectedIndices.value)
    const remaining = await DeleteNodes(indices)
    nodes.value = remaining || []
    clearResults()
    selectedIndices.value.clear()
    progress.value = { total: nodes.value.length, done: 0, passed: 0, running: false }
  }

  return {
    nodes,
    results,
    running,
    progress,
    ipLookupProgress,
    logs,
    settings,
    selectedIndices,
    passingNodes,
    selectedCount,
    getResult,
    setupListeners,
    teardownListeners,
    addLog,
    toggleSelect,
    selectAll,
    deselectAll,
    selectFailed,
    selectPassed,
    importFromText,
    importFromFile,
    importFromSubscription,
    importMultipleSubscriptions,
    importMultipleFiles,
    startTest,
    stopTest,
    updateSettings,
    loadSettings,
    exportClashYAML,
    exportNodeLinks,
    exportYAMLFlow,
    exportYAMLFlowFiltered,
    exportNodeLinksFiltered,
    getAvailableProxyTypes,
    clearAllNodes,
    deleteSelectedNodes,
  }
})
