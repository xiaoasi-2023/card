<script setup lang="ts">
import { computed } from 'vue'
import { orderStatus } from '@/utils'
const props = defineProps<{ value: string }>()
const tone = computed(() => ['completed','success','enabled','active','available','sold'].includes(props.value) ? 'success' : ['pending_payment','created','reserved'].includes(props.value) ? 'warning' : ['delivery_failed','failed','disabled','void'].includes(props.value) ? 'danger' : 'muted')
</script>
<template><span class="status" :class="`status--${tone}`"><i />{{ orderStatus[value] || value }}</span></template>
