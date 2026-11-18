<script lang="ts">
import type { VNodeChild } from 'vue'

// 上游的更新日志是 Markdown 的一个小子集：标题、有序与无序列表、段落，行内有代码、加粗和链接
export default defineComponent({
  props: {
    content: { type: String, required: true },
  },
  setup(props) {
    const inline = (text: string): VNodeChild[] =>
      text.split(/(`[^`]+`|\*\*[^*]+\*\*|\[[^\]]+\]\([^)\s]+\))/).map((part) => {
        if (part.startsWith('`')) return h('code', { class: 'font-mono' }, part.slice(1, -1))
        if (part.startsWith('**')) return h('strong', part.slice(2, -2))
        const link = part.match(/^\[([^\]]+)\]\(([^)\s]+)\)$/)
        if (link) return h('a', { href: link[2], target: '_blank', rel: 'noopener' }, link[1])
        return part
      })

    const blocks = computed(() => {
      const result: VNodeChild[] = []
      let list = null as { tag: 'ol' | 'ul'; items: VNodeChild[] } | null
      const flush = () => {
        if (list) result.push(h(list.tag, list.items))
        list = null
      }
      for (const raw of props.content.split('\n')) {
        const line = raw.trim()
        const item = line.match(/^(\d+\.|[-*])\s+(.*)$/)
        if (item) {
          const tag = /\d/.test(item[1]!) ? 'ol' : 'ul'
          if (list?.tag !== tag) flush()
          list ??= { tag, items: [] }
          list.items.push(h('li', inline(item[2]!)))
          continue
        }
        flush()
        if (!line) continue
        const heading = line.match(/^#{1,6}\s+(.*)$/)
        result.push(heading ? h('h4', inline(heading[1]!)) : h('p', inline(line)))
      }
      flush()
      return result
    })

    return () => h('div', { class: 'changelog' }, blocks.value)
  },
})
</script>

<style scoped>
.changelog {
  color: var(--color-text-secondary);
  font-size: 14px;
  line-height: 1.7;
}

.changelog h4 {
  margin: 20px 0 8px;
  color: var(--color-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.changelog h4:first-child {
  margin-top: 0;
}

.changelog p {
  margin: 0 0 8px;
}

.changelog ol,
.changelog ul {
  margin: 0;
  padding-left: 22px;
}

.changelog ol {
  list-style: decimal;
}

.changelog ul {
  list-style: disc;
}

.changelog li {
  padding-left: 2px;
}

.changelog li + li {
  margin-top: 4px;
}

.changelog li::marker {
  color: var(--color-text-tertiary);
}

.changelog code {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--color-bg-subtle);
  font-size: 0.92em;
}

.changelog a {
  color: var(--color-brand);
}
</style>
