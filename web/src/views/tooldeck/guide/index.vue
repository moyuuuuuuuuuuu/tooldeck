<template>
  <article class="guide"
    ><ElTabs v-model="activeTab" class="document-tabs"
      ><ElTabPane label="工具开发指引" name="development"
        ><div class="tab-content"
          ><header
      ><div><span class="eyebrow">DEVELOPER DOCUMENTATION</span><h1>开发者文档</h1><p>从工具包开发到 API 接入，在一个页面完成查阅。</p></div
      ><ElButton type="primary" @click="$router.push('/tooldeck/tools')">前往上传工具包</ElButton></header
    ><div class="runtime-strip"><span>支持的运行环境</span><b>PHP 8.0–8.3</b><b>Node / JS 20–23</b><b>Python 3.10–3.12</b><b>Go 1.22–1.24</b></div
    ><div class="guide-layout"
      ><aside
        ><span class="nav-label">开发路线</span
        ><a v-for="(item, i) in sections" :key="item" :href="'#step-' + i" @click.prevent="jump(i)"
          ><small>{{ String(i + 1).padStart(2, '0') }}</small
          >{{ item }}</a
        ></aside
      ><main
        ><section id="step-0"
          ><div class="step-label">STEP 01</div><h2>准备目录与输入输出</h2
          ><pre>
my-tool.zip
├── tooldeck.json
└── main.py</pre
          ><p>ZIP 根目录直接放描述文件和入口代码。代码从标准输入读取 JSON，仅向标准输出写一个 JSON 结果；日志写标准错误。不要启动常驻 HTTP 服务。</p
          ><div class="template-downloads"
            ><a v-for="template in templates" :key="template.file" :href="'/tool-templates/' + template.file" download
              ><b>{{ template.label }}</b
              ><span>下载空白 ZIP</span></a
            ></div
          ><p>空白模板可直接上传，也可解压后按语言习惯修改。PHP 模板使用 Composer PSR-4 自动加载，Node 和 Go 模板包含依赖/模块清单。</p><pre>{{ source }}</pre></section
        ><section id="step-1"
          ><div class="step-label">STEP 02</div><h2>描述工具和动态表单</h2><p>name 是唯一工具标识；同一工具升级请修改 version。以下最小示例生成一个“文本内容”输入框。</p><pre>{{ manifest }}</pre></section
        ><section id="step-2"
          ><div class="step-label">STEP 03</div><h2>图片和文件</h2><p>字段使用 type: "string"、format: "tooldeck-file"，ui_schema 对应字段使用 widget: "image-upload" 或 "file-upload"。前端上传后传 file_id，运行时自动转换为容器内只读路径。多文件使用 array，其 items 为上述文件字段。</p><p>将输出图片写入环境变量 TOOLDECK_OUTPUT_DIR 指定目录，结果会包含 artifacts；图片会在网页展示。不要返回宿主机路径。</p><pre>{{ imageField }}</pre
          ><FileReference /></section
        ><section id="step-3"><div class="step-label">STEP 04</div><h2>第三方服务与依赖</h2><p>上传时勾选“使用第三方服务”，填写精确域名，例如 api.example.com。只允许声明的公网域名，不支持通配符和内网目标。HTTP 客户端需遵循运行环境的 HTTP_PROXY / HTTPS_PROXY。</p><p>上传源码即可：Node 提供 package.json 和 package-lock.json；PHP 提供 composer.json 和 composer.lock；Python 的 requirements.txt 需锁定版本并附完整哈希；Go 模块提供 go.mod 和 go.sum。平台在隔离环境安装依赖、执行构建，再保存产物供运行复用。无需上传 node_modules 或 vendor。</p><p>个人工具不能引用平台服务密钥。不要将私人凭证放入工具结果或日志；表单输入会保存在执行记录中。</p></section
        ><section id="step-4"
          ><div class="step-label">STEP 05</div><h2>上传选项与通知</h2
          ><ul
            ><li>公开工具：公开时按站点设置审核，私有时无需审核。</li
            ><li>使用第三方服务：开启受域名限制的外网访问。</li
            ><li>异步通知结果：后台执行，完成后站内通知，并向已验证邮箱发邮件提醒。邮件不包含输入、日志或完整结果。</li
            ><li>允许 API 调用：开启后可使用本人 API Key / OAuth；关闭后仅限网页使用。</li></ul
          ><p>公开工具默认需管理员审核，通过后所有登录用户可见；私有工具不需要审核，仅本人和管理员可见。同名工具不能被他人接管。ZIP 最大64 MB，展开后128 MB、最多2000个条目。执行时限1–900秒，内存64–2048 MB。</p></section
        ><section id="step-5"><div class="step-label">STEP 06</div><h2>API 调用概览</h2><p>工具详情的“API 参数”页提供当前工具的请求地址与参数说明。先在“访问凭证”创建 Key。文件需先 POST /api/v1/files（multipart file 字段），再把返回的 file_id 放入 input。</p><p>异步请求返回 run_id；使用 GET /api/v1/runs/{run_id} 查询状态。重试提交时使用相同 Idempotency-Key，避免重复执行。每次新操作使用新的 Key。完整说明见本页下方“API 调用指南”。</p></section
        ><section id="step-6"><div class="step-label">STEP 07</div><h2>构建环境与日志</h2><p>支持 PHP 8.0–8.3、Node 20–23、Python 3.10–3.12、Go 1.22–1.24；用 runtime_version 声明版本，或上传时选择。可声明 build_command（例如 npm run build），入口文件须在构建后存在。构建时不提供业务密钥，仅允许依赖仓库出站访问；私有仓库凭证暂不支持。</p><p>构建最多10分钟，1个CPU、1GB内存，产物最多512MB。工具详情“构建与日志”查看状态和失败日志，失败可重试。成功版本不可重新构建，依赖变化应上传新版本。公开工具构建成功后才可审核。</p></section
        ><section id="step-7"
          ><div class="step-label">STEP 08</div><h2>环境变量与凭证</h2><p>打开自己的工具 → 环境变量，添加变量名、说明、必填和敏感值标记。每位使用者都能设置个人配置。优先级为：当前工具个人配置 → 当前工具作者共享配置；只注入工具已声明的同名变量。作者可另行维护共享配置，并决定是否允许未配置的变量回退到共享值。个人 API Key 调用与网页使用同一份个人配置。</p><p>代码包可声明 env 数组：每项包含 name、description、required、sensitive；env_mode 取 developer 或 user。ZIP 不应携带真实凭证。每次上传独立读取 env；未声明或为空数组时，表示该版本不需要环境变量。配置值按工具标识与提供方式保留。</p
          ><pre>
PHP: getenv('IMAGE_API_KEY')
Node: process.env.IMAGE_API_KEY
Python: os.environ['IMAGE_API_KEY']
Go: os.Getenv("IMAGE_API_KEY")</pre
          ><p>配置加密保存、不回显，缺失必填项时不能运行。日志会遮盖配置原值；代码仍应避免把凭证输出到结果、文件或发送给非预期第三方。</p></section
        ><section id="step-8"><div class="step-label">REFERENCE</div><h2>配置参数速查</h2><ConfigReference /></section><section id="step-9"><div class="step-label">STREAMING</div><h2>SSE 流式输出</h2><StreamGuide /></section></main></div></div></ElTabPane
      ><ElTabPane label="API 指南" name="api"><div class="tab-content"><ApiGuide /></div></ElTabPane></ElTabs
  ></article>
</template>
<script setup lang="ts">
  import StreamGuide from '../components/StreamGuide.vue'
  import FileReference from '../components/FileReference.vue'
  import ConfigReference from '../components/ConfigReference.vue'
  import ApiGuide from '../api-guide/index.vue'
  import { ref, watch } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  defineOptions({ name: 'Guide' })
  const route = useRoute()
  const router = useRouter()
  const activeTab = ref(route.query.tab === 'api' ? 'api' : 'development')
  watch(activeTab, (tab) => router.replace({ query: tab === 'api' ? { tab: 'api' } : {} }))
  const sections = ['准备目录与输入输出', '描述工具和动态表单', '图片和文件', '第三方服务与依赖', '上传选项与通知', 'API 调用概览', '构建环境与日志', '环境变量与凭证', '配置参数速查', 'SSE 流式输出']
  const templates = [
    { label: 'PHP 8.1 + Composer', file: 'blank-php.zip' },
    { label: 'JavaScript', file: 'blank-js.zip' },
    { label: 'Node.js 22', file: 'blank-node.zip' },
    { label: 'Python 3.12', file: 'blank-python.zip' },
    { label: 'Go 1.22', file: 'blank-go.zip' }
  ]
  function jump(i: number) {
    document.getElementById('step-' + i)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
  const source = `# main.py
import sys, json
data = json.load(sys.stdin)
print(json.dumps({"text": data["text"].upper()}, ensure_ascii=False))`
  const manifest = JSON.stringify(
    {
      schema_version: 1,
      name: 'my-text-tool',
      title: '文本大写转换',
      description: '将输入文本转换为大写',
      version: '1.0.0',
      runtime: 'python',
      runtime_version: '3.12',
      entrypoint: 'main.py',
      execution: { mode: 'sync', timeout_seconds: 30, memory_mb: 128 },
      network: { enabled: false, allowed_hosts: [] },
      secrets: [],
      input_schema: {
        type: 'object',
        properties: { text: { type: 'string', title: '文本内容' } },
        required: ['text']
      },
      ui_schema: { text: { widget: 'textarea', rows: 4 } },
      output_schema: { type: 'json' }
    },
    null,
    2
  )
  const imageField = JSON.stringify(
    {
      input_schema: {
        type: 'object',
        properties: {
          prompt: { type: 'string', title: '提示词' },
          reference: { type: 'string', title: '参考图', format: 'tooldeck-file' }
        },
        required: ['prompt']
      },
      ui_schema: {
        prompt: { widget: 'textarea', rows: 4 },
        reference: {
          widget: 'image-upload',
          accept: ['image/png', 'image/jpeg'],
          max_file_size_mb: 10
        }
      }
    },
    null,
    2
  )
</script>
<style scoped>
  .guide {
    max-width: 1180px;
    margin: auto;
    line-height: 1.85;
  }
  header {
    display: flex;
    justify-content: space-between;
    gap: 24px;
    align-items: center;
    padding: 30px 0 26px;
    border-bottom: 1px solid var(--el-border-color-lighter);
  }
  .eyebrow,
  .nav-label,
  .step-label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 1.8px;
    color: #6575ad;
  }
  h1 {
    font-size: 30px;
    letter-spacing: -0.7px;
    font-weight: 700;
    margin: 9px 0;
  }
  header p {
    margin: 0;
  }
  .runtime-strip {
    display: flex;
    gap: 12px;
    align-items: center;
    flex-wrap: wrap;
    padding: 20px 0 30px;
    font-size: 12px;
  }
  .runtime-strip span {
    color: var(--el-text-color-secondary);
    margin-right: 6px;
  }
  .runtime-strip b {
    font-weight: 500;
    background: #f1f4ff;
    color: #536398;
    padding: 4px 12px;
    border-radius: 6px;
  }
  .template-downloads {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 10px;
    margin: 18px 0;
  }
  .template-downloads a {
    display: flex;
    flex-direction: column;
    padding: 13px 15px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 10px;
    text-decoration: none;
    color: #5269ef;
    background: #f8f9ff;
  }
  .template-downloads a:hover {
    border-color: #9caaee;
  }
  .template-downloads b {
    font-size: 13px;
  }
  .template-downloads span {
    font-size: 11px;
    color: var(--el-text-color-secondary);
  }
  .guide-layout {
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    gap: 40px;
  }
  aside {
    position: sticky;
    top: 24px;
    align-self: start;
    padding: 18px 0;
  }
  .nav-label {
    display: block;
    margin-bottom: 16px;
  }
  aside a {
    display: flex;
    gap: 12px;
    padding: 10px 12px;
    margin-left: -12px;
    text-decoration: none;
    color: var(--el-text-color-secondary);
    font-size: 13px;
    border-radius: 8px;
    transition: background 0.15s;
  }
  aside a:hover {
    background: #eef2ff;
    color: #5269ef;
  }
  aside small {
    color: #98a1b6;
    font-size: 11px;
  }
  section {
    padding: 30px 32px;
    margin-bottom: 24px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 16px;
    background: var(--el-bg-color);
    scroll-margin-top: 24px;
    min-width: 0;
  }
  .document-tabs :deep(.el-tabs__header) {
    margin-bottom: 24px;
  }
  .document-tabs :deep(.el-tabs__item) {
    height: 48px;
    padding: 0 24px;
    font-size: 16px;
    font-weight: 600;
  }
  .tab-content {
    min-width: 0;
  }
  h2 {
    font-size: 21px;
    font-weight: 650;
    margin: 6px 0 18px;
  }
  p {
    margin: 12px 0;
    color: var(--el-text-color-secondary);
    font-size: 14px;
  }
  pre {
    background: #f7f8fc;
    border: 1px solid #e9edf5;
    color: #33415f;
    padding: 20px 24px;
    border-radius: 10px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font:
      13px/1.8 ui-monospace,
      Consolas,
      monospace;
    margin: 18px 0;
  }
  ul {
    padding-left: 20px;
    list-style: disc;
    color: var(--el-text-color-secondary);
    font-size: 14px;
  }
  li {
    padding: 5px 0;
  }
  @media (max-width: 800px) {
    header {
      align-items: flex-start;
      flex-direction: column;
      padding-top: 12px;
    }
    h1 {
      font-size: 25px;
    }
    .guide-layout {
      grid-template-columns: minmax(0, 1fr);
      gap: 16px;
    }
    aside {
      position: static;
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      padding: 0;
    }
    .nav-label {
      display: none;
    }
    aside a {
      padding: 6px 10px;
      margin: 0;
      border: 1px solid var(--el-border-color-lighter);
    }
    section {
      padding: 22px 18px;
    }
    pre {
      padding: 16px;
    }
    .runtime-strip {
      padding-bottom: 18px;
    }
  }
</style>
