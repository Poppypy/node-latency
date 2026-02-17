<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useNodeStore } from '../stores/nodes'

const store = useNodeStore()
const parentRef = ref<HTMLElement | null>(null)

// Simple virtual scrolling implementation
const itemHeight = 40
const bottomSafePadding = 16
const bufferSize = 10
const scrollTop = ref(0)
const viewportHeight = ref(600)

function onScroll(e: Event) {
  const target = e.target as HTMLElement
  scrollTop.value = target.scrollTop
}

function getStatus(result: any) {
  if (!result) return { text: '等待', class: 'text-gray-500' }
  if (!result.done) return { text: '测试中', class: 'text-yellow-400' }
  if (result.pass) return { text: '通过', class: 'text-green-400' }
  if (result.err) return { text: '错误', class: 'text-red-400' }
  return { text: '失败', class: 'text-orange-400' }
}

const startIndex = ref(0)
const endIndex = ref(100)

function updateRange() {
  if (!parentRef.value) return
  viewportHeight.value = parentRef.value.clientHeight
  startIndex.value = Math.max(0, Math.floor(scrollTop.value / itemHeight) - bufferSize)
  endIndex.value = Math.min(
    store.nodes.length,
    Math.ceil((scrollTop.value + viewportHeight.value) / itemHeight) + bufferSize
  )
}

onMounted(() => {
  if (parentRef.value) {
    viewportHeight.value = parentRef.value.clientHeight
  }
})

const visibleNodes = ref<{ index: number; node: any }[]>([])

watch([scrollTop, () => store.nodes.length], () => {
  updateRange()
  const nodes: { index: number; node: any }[] = []
  for (let i = startIndex.value; i < endIndex.value && i < store.nodes.length; i++) {
    nodes.push({ index: i, node: store.nodes[i] })
  }
  visibleNodes.value = nodes
}, { immediate: true })

function isSelected(index: number) {
  return store.selectedIndices.has(index)
}

function toggleSelect(index: number) {
  store.toggleSelect(index)
}

function handleRowClick(index: number, event: MouseEvent) {
  if (event.ctrlKey || event.metaKey) {
    toggleSelect(index)
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Table Header -->
    <div class="bg-gray-800 text-gray-300 text-sm font-medium border-b border-gray-700">
      <div class="grid grid-cols-8">
        <div class="px-2 py-2 text-center w-10">
          <input
            type="checkbox"
            :checked="store.selectedCount === store.nodes.length && store.nodes.length > 0"
            :indeterminate="store.selectedCount > 0 && store.selectedCount < store.nodes.length"
            @change="store.selectedCount === store.nodes.length ? store.deselectAll() : store.selectAll()"
            class="rounded bg-gray-700 border-gray-600 text-blue-500"
          />
        </div>
        <div class="px-3 py-2 truncate col-span-2">名称</div>
        <div class="px-3 py-2 truncate">主机</div>
        <div class="px-3 py-2 text-center">地区</div>
        <div class="px-3 py-2 text-right">平均(ms)</div>
        <div class="px-3 py-2 text-right">最大(ms)</div>
        <div class="px-3 py-2 text-center">状态</div>
      </div>
    </div>

    <!-- Scroll Area -->
    <div ref="parentRef" class="flex-1 overflow-y-auto overflow-x-hidden pb-2 min-h-0" @scroll="onScroll">
      <div :style="{ height: `${store.nodes.length * itemHeight + bottomSafePadding}px`, position: 'relative' }">
        <div
          v-for="{ index, node } in visibleNodes"
          :key="index"
          :style="{
            position: 'absolute',
            top: `${index * itemHeight}px`,
            height: `${itemHeight}px`,
            width: '100%'
          }"
          :class="[
            'grid grid-cols-8 border-b border-gray-800 text-sm cursor-pointer transition-colors',
            isSelected(index) ? 'bg-blue-900/30 hover:bg-blue-900/50' : 'hover:bg-gray-800/50'
          ]"
          @click="handleRowClick(index, $event)"
        >
          <div class="px-2 py-2 text-center w-10">
            <input
              type="checkbox"
              :checked="isSelected(index)"
              @change.stop="toggleSelect(index)"
              @click.stop
              class="rounded bg-gray-700 border-gray-600 text-blue-500"
            />
          </div>
          <div class="px-3 py-2 truncate col-span-2" :title="node?.name">{{ node?.name || '-' }}</div>
          <div class="px-3 py-2 truncate text-gray-400" :title="node?.host">{{ node?.host || '-' }}</div>
          <div class="px-3 py-2 truncate text-gray-400">{{ node?.region || '-' }}</div>
          <div class="px-3 py-2 text-right font-mono">
            <span v-if="store.getResult(index)?.done" :class="store.getResult(index)?.pass ? 'text-green-400' : 'text-gray-500'">
              {{ store.getResult(index)?.avgMs || '-' }}
            </span>
            <span v-else class="text-gray-600">-</span>
          </div>
          <div class="px-3 py-2 text-right font-mono">
            <span v-if="store.getResult(index)?.done" class="text-gray-500">{{ store.getResult(index)?.maxMs || '-' }}</span>
            <span v-else class="text-gray-600">-</span>
          </div>
          <div class="px-3 py-2 text-center">
            <span :class="getStatus(store.getResult(index)).class">{{ getStatus(store.getResult(index)).text }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-if="store.nodes.length === 0" class="flex-1 flex items-center justify-center text-gray-500">
      <p>暂无节点，请先导入</p>
    </div>

    <!-- Selection Actions -->
    <div v-if="store.nodes.length > 0" class="bg-gray-800 border-t border-gray-700 px-3 py-2 flex-shrink-0">
      <div class="flex items-center justify-between text-xs">
        <div class="text-gray-400">
          共 {{ store.nodes.length }} 个节点，已选 {{ store.selectedCount }} 个
        </div>
        <div class="flex gap-2">
          <button @click="store.selectAll()" class="text-blue-400 hover:text-blue-300">全选</button>
          <button @click="store.deselectAll()" class="text-gray-400 hover:text-gray-300">取消</button>
          <button @click="store.selectPassed()" class="text-green-400 hover:text-green-300">选通过</button>
          <button @click="store.selectFailed()" class="text-red-400 hover:text-red-300">选失败</button>
        </div>
      </div>
    </div>
  </div>
</template>
