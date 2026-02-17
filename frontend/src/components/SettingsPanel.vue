<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useNodeStore } from '../stores/nodes'

const store = useNodeStore()
const collapsed = ref(false)

// Settings using milliseconds for UI, converted to ns for backend
const settings = ref({
  attempts: 3,
  thresholdMs: 1500,
  timeoutMs: 1500,
  concurrency: 32,
  requireAll: true,
  stopOnFail: true,
  dedup: true,
  useCoreTest: false,
  useBatchMode: false,
  ipRename: false,
  ipNameFmt: '{region}-{random}',
})

// Convert ms to nanoseconds for Go time.Duration
function toNs(ms: number): number {
  return ms * 1_000_000
}

async function saveSettings() {
  const goSettings = {
    Attempts: settings.value.attempts,
    Threshold: toNs(settings.value.thresholdMs),
    Timeout: toNs(settings.value.timeoutMs),
    Concurrency: settings.value.concurrency,
    RequireAll: settings.value.requireAll,
    StopOnFail: settings.value.stopOnFail,
    Dedup: settings.value.dedup,
    Rename: true,
    RenameFmt: '{region} {name}',
    RegionRules: [],
    ExcludeEnabled: false,
    ExcludeKeywords: [],
    LatencyName: false,
    LatencyFmt: '{avg}ms',
    IPRename: settings.value.ipRename,
    IPLookupURL: 'http://ip-api.com/json/{ip}?fields=status,message,country,regionName,city,isp,org,as,hosting,proxy,mobile,query',
    IPLookupTimeout: toNs(3000),
    IPNameFmt: settings.value.ipNameFmt,
    UseCoreTest: settings.value.useCoreTest,
    UseBatchMode: settings.value.useBatchMode,
    CorePath: '',
    CoreTestURL: 'https://www.gstatic.com/generate_204',
    CoreStartTimeout: 90_000_000_000,
  }

  try {
    await store.updateSettings(goSettings as any)
    store.addLog('设置已保存')
  } catch (err: any) {
    store.addLog(`保存失败: ${err}`)
  }
}

onMounted(() => {
  store.loadSettings()
})
</script>

<template>
  <div class="bg-gray-800 rounded-lg p-4">
    <div class="flex items-center justify-between mb-3 cursor-pointer" @click="collapsed = !collapsed">
      <h2 class="text-sm font-semibold text-gray-300">测试设置</h2>
      <svg class="w-4 h-4 text-gray-400 transition-transform" :class="collapsed ? '' : 'rotate-180'" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </div>

    <div v-show="!collapsed" class="space-y-3 text-xs">
      <!-- Basic Settings -->
      <div class="grid grid-cols-2 gap-2">
        <div>
          <label class="block text-gray-400 mb-1">测试次数</label>
          <input v-model.number="settings.attempts" type="number" min="1" max="10" class="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1.5 focus:border-blue-500 focus:outline-none" />
        </div>
        <div>
          <label class="block text-gray-400 mb-1">阈值 (ms)</label>
          <input v-model.number="settings.thresholdMs" type="number" min="50" class="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1.5 focus:border-blue-500 focus:outline-none" />
        </div>
        <div>
          <label class="block text-gray-400 mb-1">超时 (ms)</label>
          <input v-model.number="settings.timeoutMs" type="number" min="500" class="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1.5 focus:border-blue-500 focus:outline-none" />
        </div>
        <div>
          <label class="block text-gray-400 mb-1">并发数</label>
          <input v-model.number="settings.concurrency" type="number" min="1" max="256" class="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1.5 focus:border-blue-500 focus:outline-none" />
        </div>
      </div>

      <!-- Toggles -->
      <div class="space-y-2 border-t border-gray-700 pt-2">
        <label class="flex items-center gap-2 cursor-pointer">
          <input v-model="settings.requireAll" type="checkbox" class="rounded bg-gray-700 border-gray-600 text-blue-500" />
          <span class="text-gray-300">所有次数均需低于阈值</span>
        </label>
        <label class="flex items-center gap-2 cursor-pointer">
          <input v-model="settings.stopOnFail" type="checkbox" class="rounded bg-gray-700 border-gray-600 text-blue-500" />
          <span class="text-gray-300">失败提前停止</span>
        </label>
        <label class="flex items-center gap-2 cursor-pointer">
          <input v-model="settings.dedup" type="checkbox" class="rounded bg-gray-700 border-gray-600 text-blue-500" />
          <span class="text-gray-300">按协议+主机+端口去重</span>
        </label>
      </div>

      <!-- IP Rename -->
      <div class="border-t border-gray-700 pt-2">
        <label class="flex items-center gap-2 cursor-pointer mb-2">
          <input v-model="settings.ipRename" type="checkbox" class="rounded bg-gray-700 border-gray-600 text-blue-500" />
          <span class="text-gray-300">根据 IP 地理位置重命名</span>
        </label>
        <div v-if="settings.ipRename" class="space-y-2">
          <input v-model="settings.ipNameFmt" type="text" placeholder="命名格式" class="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1.5 focus:border-blue-500 focus:outline-none" />
          <p class="text-gray-500 text-[10px]">
            可用: {region} {city} {country} {isp} {random}
          </p>
        </div>
      </div>

      <!-- Core Test -->
      <div class="border-t border-gray-700 pt-2">
        <label class="flex items-center gap-2 cursor-pointer mb-2">
          <input v-model="settings.useCoreTest" type="checkbox" class="rounded bg-gray-700 border-gray-600 text-blue-500" />
          <span class="text-gray-300">使用 Mihomo 内核真实测试</span>
        </label>
        <div v-if="settings.useCoreTest" class="ml-4 space-y-2">
          <label class="flex items-center gap-2 cursor-pointer">
            <input v-model="settings.useBatchMode" type="checkbox" class="rounded bg-gray-700 border-gray-600 text-blue-500" />
            <span class="text-gray-300 text-[11px]">分批模式（大量节点时启用）</span>
          </label>
          <p class="text-gray-500 text-[10px]">
            分批模式每次处理 200 个节点，适合超过 500 个节点的情况
          </p>
        </div>
      </div>

      <button @click="saveSettings" class="w-full py-2 bg-green-600 hover:bg-green-500 rounded text-xs font-medium transition-colors">
        保存设置
      </button>
    </div>
  </div>
</template>
