<template>
  <article class="guide"
    ><ElTabs v-model="activeTab" class="document-tabs"
      ><ElTabPane label="工具开发指引" name="development"
        ><div class="tab-content"
          ><header class="tooldeck-page-hero"
            ><div
              ><span class="eyebrow">TOOL DEVELOPMENT GUIDE</span><h1>工具开发指引</h1
              ><p>从第一个代码包开始，接入表单、文件与第三方服务。</p></div
            ><ElButton type="primary" @click="$router.push('/tooldeck/tools')"
              >前往上传工具包</ElButton
            ></header
          ><div class="runtime-strip"
            ><span>支持的运行环境</span><b>PHP 8.0–8.3</b><b>Node / JS 20–23</b
            ><b>Python 3.10–3.12</b><b>Go 1.22–1.24</b></div
          ><div class="guide-layout"
            ><aside
              ><span class="nav-label">开发路线</span
              ><a
                v-for="(item, i) in sections"
                :key="item"
                :href="'#step-' + i"
                @click.prevent="jump(i)"
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
                ><p
                  >ZIP 根目录直接放描述文件和入口代码。代码从标准输入读取 JSON，仅向标准输出写一个
                  JSON 结果；日志写标准错误。不要启动常驻 HTTP 服务。</p
                ><div class="template-downloads"
                  ><div v-for="template in templates" :key="template.file" class="template-card"
                    ><b>{{ template.label }}</b
                    ><div
                      ><a :href="'/tool-templates/' + template.file" download>下载 ZIP</a
                      ><a :href="template.source" target="_blank" rel="noopener noreferrer"
                        >查看版本源码</a
                      ></div
                    ></div
                  ></div
                ><p
                  >空白模板可直接上传，也可从独立版本标签下载；标签根目录就是工具包，不包含 ToolDeck
                  平台本体。PHP 使用 Composer PSR-4，Node 使用 npm/ES Module，Go 使用 Go
                  Modules。</p
                ><pre>{{ source }}</pre></section
              ><section id="step-1"
                ><div class="step-label">STEP 02</div><h2>描述工具和动态表单</h2
                ><p
                  >name 是唯一工具标识；同一工具升级请修改
                  version。以下最小示例生成一个“文本内容”输入框。</p
                ><pre>{{ manifest }}</pre></section
              ><section id="step-2"
                ><div class="step-label">STEP 03</div><h2>图片和文件</h2
                ><p
                  >字段使用 type: "string"、format: "tooldeck-file"，ui_schema 对应字段使用 widget:
                  "image-upload" 或 "file-upload"。前端上传后传
                  file_id，运行时自动转换为容器内只读路径。多文件使用 array，其 items
                  为上述文件字段。</p
                ><p
                  >将输出图片写入环境变量 TOOLDECK_OUTPUT_DIR 指定目录，结果会包含
                  artifacts；图片会在网页展示。不要返回宿主机路径。</p
                ><pre>{{ imageField }}</pre
                ><FileReference /></section
              ><section id="step-3"
                ><div class="step-label">STEP 04</div><h2>第三方服务与依赖</h2
                ><p
                  >上传时勾选“使用第三方服务”，填写精确域名，例如
                  api.example.com。只允许声明的公网域名，不支持通配符和内网目标。HTTP
                  客户端需遵循运行环境的 HTTP_PROXY / HTTPS_PROXY。</p
                ><p
                  >上传源码即可：Node 提供 package.json 和 package-lock.json；PHP 提供 composer.json
                  和 composer.lock；Python 的 requirements.txt 需锁定版本并附完整哈希；Go 模块提供
                  go.mod 和 go.sum。平台在隔离环境安装依赖、执行构建，再保存产物供运行复用。无需上传
                  node_modules 或 vendor。</p
                ><p
                  >个人工具不能引用平台服务密钥。不要将私人凭证放入工具结果或日志；表单输入会保存在执行记录中。</p
                ></section
              ><section id="step-4"
                ><div class="step-label">STEP 05</div><h2>上传选项</h2
                ><ul
                  ><li>公开工具：公开时按站点设置审核，私有时无需审核。</li
                  ><li>使用第三方服务：开启受域名限制的外网访问。</li
                  ><li
                    >执行模式：同步直接等待结果；异步 API 调用必须传入
                    callback_url，由平台完成后回调。</li
                  ><li
                    >允许 API 调用：开启后可使用本人 API Key / OAuth；关闭后仅限网页使用。</li
                  ></ul
                ><p
                  >公开工具默认需管理员审核，通过后所有登录用户可见；私有工具不需要审核，仅本人和管理员可见。同名工具不能被他人接管。ZIP
                  最大64 MB，展开后128 MB、最多2000个条目。执行时限1–900秒，内存64–2048 MB。</p
                ></section
              ><section id="step-5"
                ><div class="step-label">STEP 06</div><h2>API 调用概览</h2
                ><p
                  >工具详情的“API 参数”页提供当前工具的请求地址与参数说明。先在“访问凭证”创建
                  Key。文件需先 POST /api/v1/files（multipart file 字段），再把返回的 file_id 放入
                  input。</p
                ><p
                  >异步 API 请求必须传入 HTTPS callback_url；任务结束后平台 POST 结果。回调失败会按
                  3、30、300、3000、300000
                  秒重试，全部失败后才产生站内信，不发送完成邮件。重试提交时使用相同
                  Idempotency-Key，避免重复执行。</p
                ></section
              ><section id="step-6"
                ><div class="step-label">STEP 07</div><h2>构建环境与日志</h2
                ><p
                  >支持 PHP 8.0–8.3、Node 20–23、Python 3.10–3.12、Go 1.22–1.24；用 runtime_version
                  声明版本，或上传时选择。可声明 build_command（例如 npm run
                  build），入口文件须在构建后存在。构建时不提供业务密钥，仅允许依赖仓库出站访问；私有仓库凭证暂不支持。</p
                ><p
                  >构建最多10分钟，1个CPU、1GB内存，产物最多512MB。工具详情“构建与日志”查看状态和失败日志，失败可重试。成功版本不可重新构建，依赖变化应上传新版本。公开工具构建成功后才可审核。</p
                ></section
              ><section id="step-7"
                ><div class="step-label">STEP 08</div><h2>环境变量与凭证</h2
                ><p
                  >打开自己的工具 →
                  环境变量，添加变量名、说明、必填和敏感值标记。每位使用者都能设置个人配置。优先级为：当前工具个人配置
                  →
                  当前工具作者共享配置；只注入工具已声明的同名变量。作者可另行维护共享配置，并决定是否允许未配置的变量回退到共享值。个人
                  API Key 调用与网页使用同一份个人配置。</p
                ><p
                  >代码包可声明 env 数组：每项包含 name、description、required、sensitive；env_mode
                  取 developer 或 user。ZIP 不应携带真实凭证。每次上传独立读取
                  env；未声明或为空数组时，表示该版本不需要环境变量。配置值按工具标识与提供方式保留。</p
                ><pre>
PHP: getenv('IMAGE_API_KEY')
Node: process.env.IMAGE_API_KEY
Python: os.environ['IMAGE_API_KEY']
Go: os.Getenv("IMAGE_API_KEY")</pre
                ><p
                  >配置加密保存、不回显，缺失必填项时不能运行。日志会遮盖配置原值；代码仍应避免把凭证输出到结果、文件或发送给非预期第三方。</p
                ></section
              ><section id="step-8"
                ><div class="step-label">CANCELLATION</div><h2>第三方异步任务取消钩子</h2
                ><p
                  >视频生成、模型训练等第三方接口通常先返回任务 ID。设置
                  <code>execution.cancel_hook: true</code> 后，用户取消运行时，ToolDeck
                  会先在原容器中再次调用入口；取消钩子完成后才停止主任务。</p
                ><pre>{{ cancelManifest }}</pre
                ><h3>调用方如何取消任务</h3
                ><p
                  >创建任务后保存响应中的 <code>run_id</code>。网页可在执行结果中点击“取消执行”；API
                  调用方使用原凭证向取消接口发送 POST 请求：</p
                ><pre>{{ cancelRequest }}</pre
                ><p
                  >排队任务会直接变为 <code>canceled</code>。运行中任务先变为
                  <code>canceling</code>，钩子完成后变为 <code>canceled</code>；钩子失败或超时则变为
                  <code>cancel_failed</code>，错误原因位于 <code>cancel_error</code>。</p
                ><h3>工具如何取消第三方任务</h3
                ><p
                  >入口通过 <code>TOOLDECK_ACTION</code> 区分正常运行和取消。取得第三方任务 ID
                  后，应先写临时文件，再原子重命名到
                  <code>TOOLDECK_STATE_FILE</code>；取消入口读取该 JSON 并调用第三方取消接口。</p
                ><ElTabs v-model="cancelLanguage" class="code-tabs"
                  ><ElTabPane
                    v-for="example in cancelExamples"
                    :key="example.name"
                    :label="example.label"
                    :name="example.name"
                  >
                    <pre>{{ example.code }}</pre>
                  </ElTabPane></ElTabs
                ><ul
                  ><li>排队中的任务尚未调用第三方接口，取消时不会执行钩子。</li
                  ><li
                    >钩子最长运行 15 秒；成功状态为 canceled，失败状态为 cancel_failed，原因位于
                    cancel_error。</li
                  ><li>取消接口可能被重试，第三方取消操作必须设计为幂等。</li
                  ><li>钩子输出不会写入公开日志；不要输出密钥、签名或第三方完整响应。</li></ul
                ></section
              ><section id="step-9"
                ><div class="step-label">REFERENCE</div><h2>配置参数速查</h2
                ><ConfigReference /></section
              ><section id="step-10"
                ><div class="step-label">STREAMING</div><h2>SSE 流式输出</h2
                ><StreamGuide /></section></main></div></div></ElTabPane
      ><ElTabPane label="API 指南" name="api"
        ><div class="tab-content"><ApiGuide /></div></ElTabPane></ElTabs
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
  watch(activeTab, (tab) => {
    const query = { ...route.query }
    if (tab === 'api') query.tab = 'api'
    else delete query.tab
    router.replace({ query })
  })
  const sections = [
    '准备目录与输入输出',
    '描述工具和动态表单',
    '图片和文件',
    '第三方服务与依赖',
    '上传选项',
    'API 调用概览',
    '构建环境与日志',
    '环境变量与凭证',
    '异步任务取消钩子',
    '配置参数速查',
    'SSE 流式输出'
  ]
  const templates = [
    {
      label: 'PHP 8.1 + Composer',
      file: 'blank-php.zip',
      source: 'https://github.com/moyuuuuuuuuuuu/tooldeck/tree/tool-php-v1.0.0'
    },
    {
      label: 'JavaScript',
      file: 'blank-js.zip',
      source: 'https://github.com/moyuuuuuuuuuuu/tooldeck/tree/tool-js-v1.0.0'
    },
    {
      label: 'Node.js 22',
      file: 'blank-node.zip',
      source: 'https://github.com/moyuuuuuuuuuuu/tooldeck/tree/tool-node-v1.0.0'
    },
    {
      label: 'Python 3.12',
      file: 'blank-python.zip',
      source: 'https://github.com/moyuuuuuuuuuuu/tooldeck/tree/tool-python-v1.0.0'
    },
    {
      label: 'Go 1.22',
      file: 'blank-go.zip',
      source: 'https://github.com/moyuuuuuuuuuuu/tooldeck/tree/tool-go-v1.0.0'
    }
  ]
  const cancelLanguage = ref('php')
  const cancelExamples = [
    {
      name: 'php',
      label: 'PHP',
      code: `interface CancelableToolInterface extends ToolInterface
{
    public function cancel(array $input, array $state): void;
}

if (getenv('TOOLDECK_ACTION') === 'cancel') {
    $application->cancel($input, $state);
}`
    },
    {
      name: 'js',
      label: 'JavaScript',
      code: `class Application {
  run(input) { /* 返回结果 */ }
  cancel(input, state) { /* 取消 state.provider_task_id */ }
}`
    },
    {
      name: 'node',
      label: 'Node.js',
      code: `export class Tool {
  run(input) { throw new Error('not implemented') }
  cancel(input, state) { throw new Error('not implemented') }
}

export class Application extends Tool { /* 实现两个方法 */ }`
    },
    {
      name: 'python',
      label: 'Python',
      code: `class ToolInterface(ABC):
    @abstractmethod
    def run(self, input_data): ...

    @abstractmethod
    def cancel(self, input_data, state): ...`
    },
    {
      name: 'go',
      label: 'Go',
      code: `type Tool interface {
    Run(Input) (Output, error)
    Cancel(Input, State) error
}`
    }
  ]
  const cancelManifest = JSON.stringify(
    { execution: { mode: 'async', timeout_seconds: 900, memory_mb: 256, cancel_hook: true } },
    null,
    2
  )
  const cancelRequest = `curl -X POST 'https://你的域名/api/v1/runs/RUN_ID/cancel' \\
  -H 'X-API-Key: YOUR_API_KEY'

# 查询取消进度和最终状态
curl 'https://你的域名/api/v1/runs/RUN_ID' \\
  -H 'X-API-Key: YOUR_API_KEY'`
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
  .guide header.tooldeck-page-hero {
    display: block;
    padding: 32px;
    margin-bottom: 30px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 20px;
    background: linear-gradient(
      115deg,
      var(--el-color-primary-light-9),
      var(--el-bg-color) 65%,
      var(--el-color-success-light-9)
    );
  }
  .eyebrow,
  .nav-label,
  .step-label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 1.8px;
    color: #6575ad;
  }
  .eyebrow {
    font-size: 11px;
    letter-spacing: 2px;
    color: var(--el-color-primary);
  }
  .guide header h1 {
    font-size: 32px;
    letter-spacing: -0.7px;
    font-weight: 700;
    margin: 9px 0;
  }
  .guide header p {
    max-width: 760px;
    margin: 12px 0;
    line-height: 1.8;
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
    background: var(--el-fill-color-light);
    color: var(--el-color-primary);
    padding: 4px 12px;
    border-radius: 6px;
  }
  .template-downloads {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 10px;
    margin: 18px 0;
  }
  .template-card {
    display: flex;
    flex-direction: column;
    padding: 13px 15px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 10px;
    background: var(--el-fill-color-light);
  }
  .template-card:hover {
    border-color: var(--el-color-primary-light-5);
  }
  .template-card b {
    font-size: 13px;
  }
  .template-card div {
    display: flex;
    gap: 12px;
    margin-top: 5px;
  }
  .template-card a {
    color: var(--el-color-primary);
    font-size: 11px;
    text-decoration: none;
  }
  .guide-layout {
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    gap: 40px;
  }
  aside {
    position: sticky;
    top: 88px;
    align-self: start;
    max-height: calc(100vh - 112px);
    overflow-y: auto;
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
    margin-left: 0;
    text-decoration: none;
    color: var(--el-text-color-secondary);
    font-size: 13px;
    border-radius: 8px;
    transition: background 0.15s;
  }
  aside a:hover {
    background: var(--el-color-primary-light-9);
    color: var(--el-color-primary);
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
  .document-tabs :deep(.el-tabs__content) {
    overflow: visible;
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
    background: var(--el-fill-color-light);
    border: 1px solid var(--el-border-color-lighter);
    color: var(--el-text-color-primary);
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
    .guide header.tooldeck-page-hero {
      padding: 26px 22px;
    }
    .guide header h1 {
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
      max-height: none;
      overflow: visible;
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
