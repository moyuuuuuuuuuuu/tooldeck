<template>
  <div class="field">
    <label>{{ schema.title || label }} <span v-if="required" class="required">*</span></label>
    <p v-if="schema.description" class="hint">{{ schema.description }}</p>
    <template v-if="schema.type === 'object'">
      <DynamicField v-for="(child,key) in schema.properties" :key="key" :label="String(key)" :schema="child" :ui="{}" :required="schema.required?.includes(String(key))" :model-value="modelValue?.[key]" @update:model-value="value => emit('update:modelValue', {...(modelValue || {}), [key]: value})" />
    </template>
    <template v-else-if="isFile">
      <ElUpload :auto-upload="false" :show-file-list="false" :accept="ui.accept?.join(',')" :on-change="upload" :disabled="uploading || files.length >= (schema.maxItems || 1)">
        <ElButton :loading="uploading">{{ ui.widget === 'image-upload' ? '上传参考图' : '上传文件' }}</ElButton>
      </ElUpload>
      <div class="uploads"><div v-for="file in files" :key="file.id" class="upload-item"><img v-if="file.preview" :src="file.preview" :alt="file.name"/><span>{{ file.name }}</span><ElButton text type="danger" @click="remove(file.id)">移除</ElButton></div></div>
      <p class="hint">最多 {{ schema.type === 'array' ? (schema.maxItems || 1) : 1 }} 个，每个不超过 {{ ui.max_file_size_mb || 20 }} MB</p>
    </template>
    <ElSwitch v-else-if="schema.type === 'boolean'" :model-value="Boolean(modelValue)" @update:model-value="emit('update:modelValue', $event)"/>
    <ElInputNumber v-else-if="schema.type === 'number' || schema.type === 'integer'" :model-value="modelValue" :min="schema.minimum" :max="schema.maximum" :precision="schema.type === 'integer' ? 0 : undefined" @update:model-value="emit('update:modelValue', $event)"/>
    <ElRadioGroup v-else-if="schema.enum && ui.widget === 'radio'" :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)"><ElRadioButton v-for="option in schema.enum" :key="String(option)" :value="option">{{ option }}</ElRadioButton></ElRadioGroup>
    <ElSelect v-else-if="schema.enum || schema.items?.enum" :multiple="schema.type === 'array'" :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)"><ElOption v-for="option in (schema.enum || schema.items?.enum)" :key="String(option)" :label="String(option)" :value="option"/></ElSelect>
    <ElInput v-else-if="schema.type === 'array' || ui.widget === 'json'" type="textarea" :rows="5" :model-value="JSON.stringify(modelValue ?? [],null,2)" @change="parseJSON"/>
    <ElInput v-else :type="ui.widget === 'textarea' ? 'textarea' : 'text'" :rows="ui.rows || 4" :placeholder="ui.placeholder" :maxlength="schema.maxLength" :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)"/>
  </div>
</template>
<script setup lang="ts">
import {computed, ref, onBeforeUnmount} from 'vue'
import {ElMessage} from 'element-plus'
import type {UploadFile} from 'element-plus'
import {td, type Field} from '@/api/tooldeck'
defineOptions({name:'DynamicField'})
const props=defineProps<{schema:Field; ui:any; label:string; required?:boolean; modelValue:any}>()
const emit=defineEmits(['update:modelValue','uploading'])
const isFile=computed(()=>props.schema.format==='tooldeck-file'||props.schema.items?.format==='tooldeck-file')
const uploading=ref(false)
const files=ref<{id:string;name:string;preview?:string}[]>([])
async function upload(file:UploadFile){
 if(!file.raw)return
 const max=props.ui.max_file_size_mb||20
 if(file.raw.size>max*1024*1024){ElMessage.error(`文件超过 ${max} MB`);return}
 if(props.ui.accept?.length&&!props.ui.accept.includes(file.raw.type)){ElMessage.error('文件格式不支持');return}
 uploading.value=true;emit('uploading',true)
 try{const result=await td.upload(file.raw);files.value.push({id:result.file_id,name:file.name,preview:file.raw.type.startsWith('image/')?URL.createObjectURL(file.raw):undefined});update()}catch{}finally{uploading.value=false;emit('uploading',false)}
}
function update(){emit('update:modelValue',props.schema.type==='array'?files.value.map(f=>f.id):files.value[0]?.id)}
function remove(id:string){const f=files.value.find(f=>f.id===id);if(f?.preview)URL.revokeObjectURL(f.preview);files.value=files.value.filter(f=>f.id!==id);update()}
function parseJSON(value:string){try{emit('update:modelValue',JSON.parse(value))}catch{ElMessage.error('请输入有效 JSON')}}
onBeforeUnmount(()=>files.value.forEach(f=>f.preview&&URL.revokeObjectURL(f.preview)))
</script>
<style scoped>
.field{margin-bottom:24px}.field label{display:block;font-weight:600;margin-bottom:10px}.required{color:var(--el-color-danger)}.hint{font-size:12px;color:var(--el-text-color-secondary);margin:7px 0}.uploads{display:flex;gap:12px;flex-wrap:wrap}.upload-item{margin-top:12px;border:1px solid var(--el-border-color);padding:8px;border-radius:8px;display:flex;align-items:center;gap:8px}.upload-item img{width:70px;height:70px;object-fit:cover;border-radius:6px}
</style>
