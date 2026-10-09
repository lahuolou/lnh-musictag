<script setup>
import { t } from '../i18n.js'

defineProps({ open: Boolean, data: Object })
const emit = defineEmits(['close', 'later', 'view'])

function view() {
  if (data.value?.url) window.open(data.value.url, '_blank', 'noopener');
  emit('view');
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="modal-mask" @click.self="emit('close')">
      <div class="modal small">
        <div class="h3">⬆ {{ t('newVerTitle') }}</div>
        <p class="muted" style="margin:10px 0;line-height:1.7">
          {{ t('newVerFound', { cur: data?.current, latest: data?.latest }) }}
        </p>
        <div class="modal-foot">
          <button class="ghost" @click="emit('later')">{{ t('newVerLater') }}</button>
          <button @click="view">{{ t('newVerView') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
