<?php
try {
 $in=json_decode(stream_get_contents(STDIN),true,32,JSON_THROW_ON_ERROR);
 if(!is_array($in)) throw new Exception('输入必须是对象');

 $text=$in['text'];$form=$in['mode']==='表单编码（空格转 +）';
 if(!in_array($in['mode'],['标准编码（空格转 %20）','表单编码（空格转 +）'],true))throw new Exception('无效编码模式');
 if($in['operation']==='编码')$value=$form?urlencode($text):rawurlencode($text);
 elseif($in['operation']==='解码') {
  if(preg_match('/%(?![0-9A-Fa-f]{2})/',$text))throw new Exception('无效的百分号编码');
  $value=$form?urldecode($text):rawurldecode($text);
  if(!preg_match('//u',$value))throw new Exception('解码结果不是 UTF-8 文本');
 }else throw new Exception('无效操作');
 $result=['result'=>$value];

 echo json_encode($result,JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_THROW_ON_ERROR);
} catch(Throwable $e) { fwrite(STDERR,$e->getMessage()); exit(1); }
