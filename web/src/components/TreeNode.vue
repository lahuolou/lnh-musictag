<script setup>
defineOptions({ name: 'TreeNode' })
const props = defineProps({
  node: Object,
  depth: Number,
  collapsed: Set,
  selected: Set,
  toggle: Function,
  openTrack: Function,
  toggleSel: Function,
  artists: Function,
  folderCount: Function
})
</script>

<template>
  <div class="tnode" :style="{ paddingLeft: (depth * 16) + 'px' }">
    <template v-if="node.dir">
      <span class="tog" @click="toggle(node)">{{ collapsed.has(node.path) ? '▸' : '▾' }}</span>
      <span class="di">📁</span>
      <span class="dn" @click="toggle(node)">{{ node.name }}</span>
      <span class="muted cnt">{{ folderCount(node) }} 首</span>
      <template v-if="!collapsed.has(node.path)">
        <TreeNode v-for="c in node.children" :key="c.name + '|' + c.path" :node="c" :depth="depth + 1"
          :collapsed="collapsed" :selected="selected"
          :toggle="toggle" :openTrack="openTrack" :toggleSel="toggleSel"
          :artists="artists" :folderCount="folderCount" />
      </template>
    </template>
    <template v-else>
      <span class="tog"></span>
      <input type="checkbox" class="tchk" :checked="selected.has(node.track.id)" @change="toggleSel(node.track, $event)" />
      <span class="tt" :title="node.track.path" @click="openTrack(node.track)">{{ node.track.tags.TITLE || node.name }}</span>
      <span class="muted ar">{{ artists(node.track) }}</span>
    </template>
  </div>
</template>
