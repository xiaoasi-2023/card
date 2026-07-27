<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

onMounted(() => useAuthStore().restore())
</script>

<template><RouterView /></template>
