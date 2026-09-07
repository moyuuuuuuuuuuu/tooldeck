<?php
try {
 $in=json_decode(stream_get_contents(STDIN),true,32,JSON_THROW_ON_ERROR);
 if(!is_array($in)) throw new Exception('输入必须是对象');

 if($in['operation']==='序列化') {
  $v=json_decode($in['text'],true,32,JSON_THROW_ON_ERROR|JSON_BIGINT_AS_STRING);
  $result=['result'=>serialize($v)];
 } elseif($in['operation']==='反序列化') {
  set_error_handler(function($severity,$message){throw new Exception('无效的序列化内容');});
  try{$v=unserialize($in['text'],['allowed_classes'=>false,'max_depth'=>32]);}finally{restore_error_handler();}
  $check=function($x,$depth=0)use(&$check){if($depth>32||is_object($x))throw new Exception('不支持对象、循环引用或过深结构');if(is_array($x))foreach($x as $y)$check($y,$depth+1);};$check($v);
  if(serialize($v)!==$in['text'])throw new Exception('仅接受规范且完整的 PHP 序列化内容');
  $result=['result'=>json_encode($v,JSON_UNESCAPED_UNICODE|JSON_PRETTY_PRINT|JSON_THROW_ON_ERROR)];
 }else throw new Exception('无效操作');

 echo json_encode($result,JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_THROW_ON_ERROR);
} catch(Throwable $e) { fwrite(STDERR,$e->getMessage()); exit(1); }
