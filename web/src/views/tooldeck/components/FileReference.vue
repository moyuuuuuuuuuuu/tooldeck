<template><div class="file-reference"><h3>图片与文件参数</h3><div class="scroll"><table><thead><tr><th>配置路径</th><th>说明 / 示例</th></tr></thead><tbody><tr v-for="r in rows" :key="r[0]"><td><code>{{r[0]}}</code></td><td>{{r[1]}}</td></tr></tbody></table></div><h3>从上传到返回结果</h3><ol><li>网页由上传控件提交文件；API 使用 POST /api/v1/files，multipart 字段名为 file，单文件最大 20 MB。</li><li>将上传响应中的 data.file_id 放入工具输入。多文件传 file_id 数组，不传文件 URL 或宿主机路径。</li><li>平台校验文件归属，并将标记为 tooldeck-file 的值替换为容器内只读路径，工具代码直接读取该路径。</li><li>将结果文件写入 TOOLDECK_OUTPUT_DIR。标准输出仍只返回 JSON；平台自动收集文件并写入任务的 artifacts。</li></ol><h3>多张参考图示例</h3><pre>{{ multi }}</pre><h3>输入与结果格式</h3><pre>{{ io }}</pre><p>artifacts 每项包含 file_id、name、mime、size（字节），通过 GET /api/v1/files/{file_id} 鉴权下载。产物最多 20 个、合计 64 MB，只接受普通文件，不接受符号链接。图片按 MIME 在结果页预览，其他文件提供下载。</p><p>accept 和 max_file_size_mb 是网页选择限制，不能代替工具自身对文件内容的校验。使用公共读 BOS 时，知道对象地址的人可直接读取对象。</p></div></template>
<script setup lang="ts">
const rows=[
 ['input_schema.properties.字段名.type','单文件使用 string；多文件使用 array，并定义 items。'],
 ['format / items.format','固定为 tooldeck-file。单文件在字段本身声明，多文件在 items 中声明。'],
 ['title / description','上传域名称和用途提示，例如“参考图”“最多上传 3 张”。'],
 ['required','所属 object 的必填字段名数组；多文件至少一个还需 minItems: 1。'],
 ['minItems / maxItems','多文件数量范围；请显式填写 maxItems，网页未设置时最多选择 1 个。'],
 ['ui_schema.字段名.widget','image-upload 显示参考图上传按钮；file-upload 显示文件上传按钮。文件行为由 format 识别。'],
 ['ui_schema.字段名.accept','MIME 类型数组，例如 ["image/png","image/jpeg"]；不是 *.png 这样的扩展名列表。省略时不限制网页选择类型。'],
 ['ui_schema.字段名.max_file_size_mb','网页单文件上限，默认 20 MB；可设为 10，不可突破服务端 20 MB 上限。'],
 ['ui_schema.字段名.order','顶层表单显示顺序，数字越小越靠前。'],
 ['TOOLDECK_OUTPUT_DIR','运行时注入的输出目录。用环境变量获取，不要在代码中固定 NAS 路径。']
]
const multi=JSON.stringify({input_schema:{type:'object',properties:{references:{type:'array',title:'参考图',minItems:1,maxItems:3,items:{type:'string',format:'tooldeck-file'}}},required:['references']},ui_schema:{references:{widget:'image-upload',accept:['image/png','image/jpeg'],max_file_size_mb:10,order:20}}},null,2)
const io=`调用输入：
{"input":{"references":["file_xxx","file_yyy"]}}

工具从 stdin 实际读取到的内容（路径仅为示意）：
{"references":["/job/input/file_xxx.png","/job/input/file_yyy.jpg"]}

任务结果中的文件列表（平台自动生成）：
{"artifacts":[{"file_id":"file_result","name":"result.png","mime":"image/png","size":1024}]}`
</script>
<style scoped>h3{font-size:16px;font-weight:650;margin:24px 0 12px}.scroll{overflow:auto;border:1px solid var(--el-border-color-lighter);border-radius:10px}table{width:100%;min-width:560px;border-collapse:collapse;font-size:12px;line-height:1.8}th,td{text-align:left;padding:12px;border-bottom:1px solid var(--el-border-color-lighter);vertical-align:top}th{background:var(--el-fill-color-light)}code{color:#5269ef;overflow-wrap:anywhere}td:first-child{width:36%}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#f7f8fc;color:#33415f;padding:18px;border-radius:10px;font:12px/1.8 monospace}ol{padding-left:20px;list-style:decimal}li,p{font-size:13px;line-height:1.9;color:var(--el-text-color-secondary);margin:10px 0}</style>
