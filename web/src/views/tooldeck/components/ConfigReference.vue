<template>
  <div class="reference"
    ><p class="intro"
      >按当前平台实际支持的字段整理。下面使用点号表示嵌套路径，例如 execution.mode 表示 execution
      对象中的 mode。未列出的自定义字段不应写入配置。</p
    ><ElInput v-model="query" clearable placeholder="搜索配置参数，例如 timeout、env、widget" /><div
      v-for="group in filtered"
      :key="group.title"
      class="group"
      ><h3>{{ group.title }}</h3
      ><div class="table-scroll"
        ><table
          ><thead
            ><tr><th>参数</th><th>取值 / 默认值</th><th>说明</th></tr></thead
          ><tbody
            ><tr v-for="row in group.rows" :key="row[0]"
              ><td
                ><code>{{ row[0] }}</code></td
              ><td>{{ row[1] }}</td
              ><td>{{ row[2] }}</td></tr
            ></tbody
          ></table
        ></div
      ></div
    ><ElEmpty v-if="!filtered.length" description="没有匹配的参数" :image-size="60" /><p
      class="intro"
      >ui_schema
      使用顶层输入字段名作为键。当前嵌套对象子字段按默认控件渲染；环境变量优先级为当前工具个人配置 →
      当前工具作者共享配置。</p
    ></div
  >
</template>
<script setup lang="ts">
  import { ref, computed } from 'vue'
  const query = ref('')
  const groups = [
    {
      title: '基础信息',
      rows: [
        ['schema_version', '必须为 1', '配置格式版本，与工具版本无关。'],
        ['name', '必填', '工具唯一标识；小写字母、数字、点、下划线、连字符。同名版本归同一作者。'],
        ['title / description', '建议填写', '工具显示名称与用途说明。'],
        ['version', '必填，如 1.0.0', '工具版本，字符规则同 name；修改代码或依赖时使用新版本。'],
        ['runtime', '必填', 'php、node/js、python/py、go/golang。'],
        [
          'runtime_version',
          '可选',
          'PHP 默认 8.3，Node 默认 22，Python 默认 3.12，Go 默认 1.24；支持版本见页首。'
        ],
        [
          'entrypoint',
          '必填，如 main.php',
          '包内相对文件路径，禁止绝对路径和越级路径，构建后必须存在。'
        ],
        [
          'build_command',
          '可选，最多 512 字节',
          '依赖安装后执行，例如 npm run build。构建不注入业务环境变量。'
        ]
      ]
    },
    {
      title: '执行与网络',
      rows: [
        [
          'execution.stream',
          '默认 false',
          '开启工具 delta 事件转发；网页实时显示，API 可订阅 SSE。'
        ],
        [
          'execution.mode',
          '必填：sync / async',
          '同步最多等待 20 秒，超时任务继续并返回 202；异步立即返回任务记录。'
        ],
        ['execution.timeout_seconds', '必填：1–900', '单次任务执行时限，单位秒。'],
        ['execution.memory_mb', '必填：64–2048', '单次执行容器内存上限，单位 MB。'],
        ['network.enabled', '默认 false', '是否允许受代理限制的第三方 HTTP(S) 请求。'],
        [
          'network.allowed_hosts',
          '默认 []',
          '启用网络时必须填写精确域名，例如 api.example.com；不能填写协议、路径、端口或通配符。'
        ],
        ['secrets', '默认 []', '平台服务密钥名称列表，仅管理员包可引用；普通工具请使用 env。'],
        [
          'output_schema.type',
          '建议填写 json',
          '普通工具必须输出单个合法 JSON；此字段不提供完整输出 JSON Schema 校验。文本输出仅在线运行使用。'
        ]
      ]
    },
    {
      title: '输入数据：input_schema',
      rows: [
        [
          'type',
          '根节点必须为 object',
          '子字段支持 object、array、string、number、integer、boolean。'
        ],
        [
          'properties',
          '对象字段定义',
          '键为输入参数名，值为字段 Schema；每个嵌套对象都可定义自己的 properties。'
        ],
        ['required', '默认 []', '必填字段名数组，写在字段所属的 object 中，不是每个字段的布尔值。'],
        ['title / description', '可选', '表单标签与辅助说明。'],
        ['default', '可选', '网页表单初始值；API 调用方应自行提交需要的值，不依赖网页默认值。'],
        ['enum', '可选数组', '允许的值列表；网页呈现选择控件，数组多选使用 items.enum。'],
        ['items', 'array 时必填', '数组元素 Schema，多文件使用 items.format=tooldeck-file。'],
        ['minimum / maximum', '可选数字', 'number / integer 的数值边界。'],
        ['minLength / maxLength', '可选整数', '字符串长度边界。'],
        ['minItems / maxItems', '可选整数', '数组元素数量边界。多文件建议明确设置 maxItems。'],
        [
          'format',
          '文件字段填 tooldeck-file',
          '文件字段为 string，网页上传后传 file_id，执行时转换为容器内只读文件路径。'
        ]
      ]
    },
    {
      title: '表单外观：ui_schema.字段名',
      rows: [
        [
          'widget',
          '按字段类型自动选择',
          'textarea 多行文本；radio 搭配 enum；json 编辑 JSON；image-upload / file-upload 上传。普通文本、数字、布尔、下拉由类型或 enum 决定。'
        ],
        ['placeholder', '可选文本', '输入框占位提示。'],
        ['rows', '多行文本默认 4', 'textarea 显示行数；JSON 编辑框当前固定 5 行。'],
        ['order', '可选整数', '顶层字段排序；建议使用 10、20、30 等数字。'],
        [
          'accept',
          '可选 MIME 数组',
          '例如 ["image/png","image/jpeg"]，用于网页文件选择和类型检查。'
        ],
        ['max_file_size_mb', '网页默认 20', '单文件大小上限，不能突破平台上传接口 20 MB 限制。']
      ]
    },
    {
      title: '环境变量：env 与 env_mode',
      rows: [
        [
          'env_mode',
          '默认 developer',
          'developer 允许回退作者共享配置；user 仅使用使用者配置（个人中心按工具分组）。'
        ],
        [
          'env',
          '最多 50 项',
          '变量定义数组，不要在包内写真实值。省略或填写 [] 均表示不需要环境变量，不继承旧版本定义。'
        ],
        [
          'env[].name',
          '必填',
          '大写字母开头，后接大写字母、数字、下划线，最多 64 字符；禁止 PATH、代理及运行时保留名。'
        ],
        ['env[].description', '可选，最多 300 字', '说明变量用途和获取方式。'],
        ['env[].required', '默认 false', '未配置或值为空时是否阻止执行。'],
        [
          'env[].sensitive',
          '默认 false',
          '控制工具配置界面的敏感值输入显示；所有变量实际均加密保存、不回显。'
        ]
      ]
    },
    {
      title: '上传页面选项（不写入 tooldeck.json）',
      rows: [
        [
          'file',
          '必填 ZIP',
          '根目录包含 tooldeck.json；ZIP 最大 64 MB，展开后最多 128 MB / 2000 个条目。'
        ],
        ['public', '接口默认 true', '公开时按站点设置审核；私有无需审核，仅本人和管理员可见。'],
        ['api_enabled', '接口默认 true', '是否允许 API Key / OAuth 调用，关闭后仅允许网页执行。'],
        ['third_party', '省略时保留包内设置', '覆盖 network.enabled；关闭时清空域名列表。'],
        ['allowed_hosts', '可选', '覆盖允许域名列表，上传字段使用逗号、空格或换行分隔。'],
        [
          'mode / runtime_version / build_command',
          '可选',
          '非空时覆盖包内对应字段；build_runtime 用于核对上传选择的语言与包内 runtime 是否一致。'
        ]
      ]
    }
  ]
  const filtered = computed(() =>
    groups
      .map((g) => ({
        ...g,
        rows: g.rows.filter((r) =>
          r.join(' ').toLowerCase().includes(query.value.trim().toLowerCase())
        )
      }))
      .filter((g) => g.rows.length)
  )
</script>
<style scoped>
  .intro {
    font-size: 13px;
    line-height: 1.9;
    color: var(--el-text-color-secondary);
    margin: 12px 0 18px;
  }
  .group {
    margin-top: 28px;
  }
  h3 {
    font-size: 16px;
    font-weight: 650;
    margin-bottom: 12px;
  }
  .table-scroll {
    overflow: auto;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 10px;
  }
  table {
    border-collapse: collapse;
    width: 100%;
    min-width: 610px;
    font-size: 12px;
    line-height: 1.8;
  }
  th {
    text-align: left;
    background: var(--el-fill-color-light);
    font-weight: 600;
  }
  th,
  td {
    padding: 13px 14px;
    border-bottom: 1px solid var(--el-border-color-lighter);
    vertical-align: top;
  }
  td:first-child {
    width: 25%;
  }
  td:nth-child(2) {
    width: 25%;
    color: var(--el-text-color-secondary);
  }
  tr:last-child td {
    border-bottom: 0;
  }
  code {
    color: #5269ef;
    overflow-wrap: anywhere;
  }
</style>
