import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useThemeStore = defineStore('theme', () => {
  const isDark = ref(true)

  function toggleTheme() {
    isDark.value = !isDark.value
    applyTheme()
    localStorage.setItem('syslog-dark', String(isDark.value))
  }

  function initTheme() {
    const saved = localStorage.getItem('syslog-dark')
    if (saved !== null) {
      isDark.value = saved === 'true'
    }
    applyTheme()
  }

  function applyTheme() {
    const root = document.documentElement

    root.classList.remove('dark-mode', 'light-mode')
    root.classList.add(isDark.value ? 'dark-mode' : 'light-mode')
  }

  return {
    isDark,
    toggleTheme,
    initTheme
  }
})
