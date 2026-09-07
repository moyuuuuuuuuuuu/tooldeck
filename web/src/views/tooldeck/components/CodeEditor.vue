<template><div class="editor"><div class="numbers"><pre :style="{transform:`translateY(-${scrollTop}px)`}">{{ numbers }}</pre></div><div class="code-area"><pre ref="highlighted" class="highlight" aria-hidden="true"><code v-html="html"></code></pre><textarea ref="input" :value="modelValue" spellcheck="false" autocapitalize="off" autocomplete="off" aria-label="代码编辑器" @input="update" @scroll="sync" @keydown.tab.prevent="indent" @keydown.ctrl.enter.prevent="$emit('run')" @keydown.meta.enter.prevent="$emit('run')"/></div></div></template>
<script setup lang="ts">
import {computed,ref} from 'vue'
import hljs from 'highlight.js/lib/core'
import php from 'highlight.js/lib/languages/php'
import javascript from 'highlight.js/lib/languages/javascript'
import python from 'highlight.js/lib/languages/python'
import go from 'highlight.js/lib/languages/go'
import 'highlight.js/styles/github-dark.css'
hljs.registerLanguage('php',php);hljs.registerLanguage('javascript',javascript);hljs.registerLanguage('python',python);hljs.registerLanguage('go',go)
const props=defineProps<{modelValue:string;language:string}>();const emit=defineEmits(['update:modelValue','run']);const input=ref<HTMLTextAreaElement>(),highlighted=ref<HTMLElement>(),scrollTop=ref(0)
const html=computed(()=>hljs.highlight(props.modelValue+'\n',{language:['node','js'].includes(props.language)?'javascript':props.language,ignoreIllegals:true}).value)
const numbers=computed(()=>Array.from({length:props.modelValue.split('\n').length},(_,i)=>i+1).join('\n'))
function update(e:Event){emit('update:modelValue',(e.target as HTMLTextAreaElement).value)}
function sync(){if(input.value&&highlighted.value){highlighted.value.scrollTop=input.value.scrollTop;highlighted.value.scrollLeft=input.value.scrollLeft;scrollTop.value=input.value.scrollTop}}
function indent(){const el=input.value;if(!el)return;const start=el.selectionStart,end=el.selectionEnd;const value=props.modelValue.slice(0,start)+'  '+props.modelValue.slice(end);emit('update:modelValue',value);requestAnimationFrame(()=>{el.selectionStart=el.selectionEnd=start+2})}
</script><style scoped>.editor{display:flex;height:460px;background:#0d1117;border-radius:12px;overflow:hidden;border:1px solid #30363d}.numbers{width:46px;flex-shrink:0;background:#111820;color:#687888;overflow:hidden;text-align:right}.numbers pre{padding:16px 10px;line-height:24px;font:14px/24px ui-monospace,SFMono-Regular,Consolas,monospace;margin:0}.code-area{position:relative;flex:1;min-width:0}.highlight,textarea{position:absolute;inset:0;width:100%;height:100%;margin:0;padding:16px;font:14px/24px ui-monospace,SFMono-Regular,Consolas,monospace;white-space:pre;overflow:auto;tab-size:2;border:0;box-sizing:border-box}.highlight{color:#c9d1d9;pointer-events:none;scrollbar-width:none}textarea{background:transparent;color:transparent;caret-color:#fff;resize:none;outline:none;-webkit-text-fill-color:transparent}textarea::selection{background:#375c88;color:transparent}.highlight code{font:inherit}</style>
