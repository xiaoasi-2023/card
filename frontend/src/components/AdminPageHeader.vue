<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>
<template><div class="admin-page-head"><div><h1>{{title}}</h1><p v-if="description">{{description}}</p></div><slot/></div></template>
