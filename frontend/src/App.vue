<script setup lang="ts">
import { onMounted, ref } from 'vue'

// Dev-environment smoke page: proves Vite -> /api proxy -> Go -> systemctl/dae all work.
const health = ref<Record<string, unknown> | null>(null)
const status = ref<Record<string, unknown> | null>(null)
const error = ref('')

async function load() {
  error.value = ''
  try {
    health.value = await (await fetch('/api/health')).json()
    status.value = await (await fetch('/api/dae/status')).json()
  } catch (e) {
    error.value = String(e)
  }
}

onMounted(load)
</script>

<template>
  <main>
    <h1>dae WebUI <small>dev</small></h1>
    <button @click="load">刷新</button>
    <p v-if="error" class="err">{{ error }}</p>
    <section>
      <h2>后端</h2>
      <pre>{{ health }}</pre>
    </section>
    <section>
      <h2>dae 状态</h2>
      <pre>{{ status }}</pre>
    </section>
  </main>
</template>

<style>
body { font-family: system-ui, sans-serif; margin: 2rem; background: #0f172a; color: #e2e8f0; }
pre { background: #1e293b; padding: 1rem; border-radius: 8px; overflow: auto; }
button { padding: .4rem 1rem; cursor: pointer; }
.err { color: #f87171; }
small { color: #94a3b8; font-size: .5em; }
</style>
