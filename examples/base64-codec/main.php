<?php
try {
 $in=json_decode(stream_get_contents(STDIN),true,32,JSON_THROW_ON_ERROR);
 if(!is_array($in)) throw new Exception('输入必须是对象');

 $text=$in['text'];
 if($in['operation']==='编码') $result=['result'=>base64_encode($text)];
 elseif($in['operation']==='解码') {
  $clean=preg_replace('/\s+/','',$text);$v=base64_decode($clean,true);
  if($v===false || rtrim(base64_encode($v),'=')!==rtrim($clean,'=')) throw new Exception('无效的 Base64');
  if(!preg_match('//u',$v)) throw new Exception('解码结果不是 UTF-8 文本');
  $result=['result'=>$v];
 } else throw new Exception('无效操作');

 echo json_encode($result,JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_THROW_ON_ERROR);
} catch(Throwable $e) { fwrite(STDERR,$e->getMessage()); exit(1); }
