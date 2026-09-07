<template>
  <div
    ><header
      ><div><h1>在线调试</h1><p>写一段代码，立即验证想法。无需创建项目或上传 ZIP。</p></div
      ><ElButton type="primary" size="large" :loading="busy" @click="runCode">▶ 运行代码</ElButton></header
    ><div class="toolbar"
      ><ElSelect v-model="language" :disabled="busy" @change="changeLanguage" style="width: 170px"><ElOption v-for="(label, key) in languages" :key="key" :value="key" :label="label" /></ElSelect><ElSelect v-model="version" :disabled="busy" style="width: 140px"><ElOption v-for="v in versions" :key="v" :value="v" :label="v" /></ElSelect><ElButton :disabled="busy" @click="example">载入示例</ElButton><ElButton @click="download">下载代码</ElButton><ElButton v-if="busy" type="danger" plain @click="stop">{{ activeRun ? '停止运行' : '停止等待' }}</ElButton
      ><span>Ctrl / ⌘ + Enter 运行</span></div
    ><div class="columns"
      ><section><CodeEditor v-model="code" :language="language" @run="runCode" /><p class="hint">单文件 / 标准库 · 代码最多64 KB · 执行最多10秒 · 禁止网络访问。JavaScript 使用 Node 引擎，不提供浏览器 DOM。</p><label>标准输入（stdin）</label><ElInput v-model="stdin" type="textarea" :rows="4" placeholder="可选：输入程序从标准输入读取的文本" maxlength="65536" /></section
      ><section class="result-panel"
        ><div class="result-title"
          ><h2>运行结果</h2><ElTag>{{ status }}</ElTag></div
        ><p v-if="activeRun">耗时 {{ activeRun.duration_ms }} ms</p><ElAlert v-if="error" :title="error" type="error" :closable="false" /><ElTabs
          ><ElTabPane label="标准输出">
            <pre>{{ activeRun?.result ?? '运行后在这里显示输出。' }}</pre></ElTabPane
          ><ElTabPane label="错误输出">
            <pre>{{ activeRun?.logs || '暂无错误输出。' }}</pre></ElTabPane
          ><ElTabPane label="环境 / 编译日志">
            <pre>{{ buildLog || '首次使用版本可能需要下载环境，准备完成后显示日志。' }}</pre>
          </ElTabPane></ElTabs
        ><p class="hint">代码和结果仅自己及管理员可访问，不会发布到工具库。运行记录保存在“我的记录”。停止等待不会中断后台环境准备。</p></section
      ></div
    ></div
  >
</template>
<script setup lang="ts">
  import { ref, computed, onBeforeUnmount } from 'vue'
  import { ElMessage } from 'element-plus'
  import request from '@/utils/http'
  import { td, type Tool, type Run } from '@/api/tooldeck'
  import CodeEditor from '../components/CodeEditor.vue'
  defineOptions({ name: 'Playground' })
  const languages: Record<string, string> = {
    php: 'PHP',
    node: 'Node.js',
    js: 'JavaScript',
    python: 'Python',
    go: 'Go / Golang'
  }
  const matrix: Record<string, string[]> = {
    php: ['8.0', '8.1', '8.2', '8.3'],
    node: ['20', '21', '22', '23'],
    js: ['20', '21', '22', '23'],
    python: ['3.10', '3.11', '3.12'],
    go: ['1.22', '1.23', '1.24']
  }
  const samples: Record<string, string> = {
    php: '<?php\n$name = trim(stream_get_contents(STDIN)) ?: "ToolDeck";\necho "Hello, " . $name . "!\\n";\necho "PHP " . PHP_VERSION . "\\n";\n',
    node: 'const fs = require("node:fs");\nconst name = fs.readFileSync(0, "utf8").trim() || "ToolDeck";\nconsole.log(`Hello, ${name}!`);\nconsole.log(process.version);\n',
    js: 'const numbers = [1, 2, 3, 4, 5];\nconsole.log("平方:", numbers.map(n => n * n));\nconsole.log("总和:", numbers.reduce((sum, n) => sum + n, 0));\n',
    python: 'import sys\nname = sys.stdin.read().strip() or "ToolDeck"\nprint(f"Hello, {name}!")\nprint(sys.version)\n',
    go: 'package main\n\nimport ("fmt"; "io"; "os"; "strings"; "runtime")\n\nfunc main() {\n  data, _ := io.ReadAll(os.Stdin)\n  name := strings.TrimSpace(string(data))\n  if name == "" { name = "ToolDeck" }\n  fmt.Printf("Hello, %s!\\n", name)\n  fmt.Println(runtime.Version())\n}\n'
  }
  const language = ref('php'),
    version = ref('8.1'),
    code = ref(samples.php),
    stdin = ref(''),
    busy = ref(false),
    activeRun = ref<Run | null>(null),
    buildLog = ref(''),
    error = ref(''),
    status = ref('等待运行')
  const versions = computed(() => matrix[language.value])
  const drafts: Record<string, string> = {}
  let lastLanguage = 'php',
    generation = 0
  function changeLanguage() {
    drafts[lastLanguage] = code.value
    code.value = drafts[language.value] ?? samples[language.value]
    lastLanguage = language.value
    version.value = language.value === 'node' || language.value === 'js' ? '22' : matrix[language.value].at(-1)!
  }
  function example() {
    code.value = samples[language.value]
  }
  function download() {
    const extension = ({ php: 'php', node: 'js', js: 'js', python: 'py', go: 'go' } as Record<string, string>)[language.value]
    const url = URL.createObjectURL(new Blob([code.value], { type: 'text/plain;charset=utf-8' }))
    const a = document.createElement('a')
    a.href = url
    a.download = 'main.' + extension
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  const delay = () => new Promise((resolve) => setTimeout(resolve, 1500))
  async function runCode() {
    if (busy.value) return
    if (new TextEncoder().encode(code.value).length > 65536) {
      ElMessage.warning('代码不能超过64 KB')
      return
    }
    const current = ++generation
    const input = stdin.value
    busy.value = true
    activeRun.value = null
    error.value = ''
    buildLog.value = ''
    status.value = '准备运行环境'
    try {
      let tool = (await td.save('playground', {
        language: language.value,
        version: version.value,
        code: code.value
      })) as Tool
      if (current !== generation) return
      if (tool.build_status === 'failed') {
        tool = (await td.save(`tools/${tool.id}/build`, {})) as Tool
      }
      while (['queued', 'building'].includes(tool.build_status || '')) {
        if (current !== generation) return
        await delay()
        if (current !== generation) return
        tool = await request.get<Tool>({ url: `/v1/tools/${tool.id}/build` })
      }
      if (current !== generation) return
      buildLog.value = tool.build_log || ''
      if (tool.build_status === 'failed') {
        error.value = tool.build_error || '编译失败'
        status.value = '编译失败'
        return
      }
      status.value = '正在运行'
      const started = await td.execute(tool.id, { stdin: input })
      if (current !== generation) {
        await td.cancel(started.run_id)
        return
      }
      activeRun.value = started
      while (['queued', 'running'].includes(activeRun.value?.status || '')) {
        if (current !== generation) return
        await delay()
        if (current !== generation) return
        const latest = await td.run(activeRun.value!.run_id)
        if (current !== generation) return
        activeRun.value = latest
      }
      status.value =
        (
          {
            succeeded: '运行完成',
            failed: '运行失败',
            timed_out: '运行超时',
            canceled: '已停止'
          } as Record<string, string>
        )[activeRun.value!.status] || activeRun.value!.status
      error.value = activeRun.value?.error || ''
    } catch (e) {
      if (current === generation) {
        status.value = '请求失败'
        error.value = e instanceof Error ? e.message : '请求失败'
      }
    } finally {
      if (current === generation) busy.value = false
    }
  }
  async function stop() {
    generation++
    busy.value = false
    status.value = '已停止等待'
    if (activeRun.value && ['queued', 'running'].includes(activeRun.value.status)) {
      activeRun.value = await td.cancel(activeRun.value.run_id)
      status.value = '已停止'
    }
  }
  onBeforeUnmount(() => {
    generation++
  })
</script>
<style scoped>
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 18px;
    margin-bottom: 24px;
  }
  h1 {
    font-size: 30px;
    font-weight: 700;
  }
  header p,
  .hint {
    color: var(--el-text-color-secondary);
    margin: 10px 0;
    line-height: 1.8;
  }
  .toolbar {
    display: flex;
    gap: 12px;
    align-items: center;
    flex-wrap: wrap;
    margin-bottom: 18px;
  }
  .toolbar span {
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }
  .columns {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 24px;
  }
  .result-panel {
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-light);
    border-radius: 12px;
    padding: 20px;
  }
  .result-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  h2 {
    font-size: 19px;
    font-weight: 600;
  }
  pre {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font:
      14px/1.7 ui-monospace,
      Consolas,
      monospace;
    max-height: 440px;
    overflow: auto;
    padding: 12px;
    background: var(--el-fill-color-light);
    min-height: 160px;
  }
  label {
    display: block;
    margin: 18px 0 10px;
  }
  @media (max-width: 900px) {
    .columns {
      grid-template-columns: 1fr;
    }
    header {
      align-items: flex-start;
    }
  }
</style>
