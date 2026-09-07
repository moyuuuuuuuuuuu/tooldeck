<?php
try {
 $in=json_decode(stream_get_contents(STDIN),true,32,JSON_THROW_ON_ERROR);
 if(!is_array($in)) throw new Exception('输入必须是对象');

 $v=json_decode($in['text'],false,64,JSON_THROW_ON_ERROR|JSON_BIGINT_AS_STRING);
 if(!in_array($in['operation'],['格式化','压缩'],true))throw new Exception('无效操作');
 $flags=JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_PRESERVE_ZERO_FRACTION|JSON_THROW_ON_ERROR;
 if($in['operation']==='格式化')$flags|=JSON_PRETTY_PRINT;
 $result=['result'=>json_encode($v,$flags)];

 echo json_encode($result,JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_THROW_ON_ERROR);
} catch(Throwable $e) { fwrite(STDERR,$e->getMessage()); exit(1); }
