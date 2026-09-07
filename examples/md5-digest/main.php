<?php
try {
 $in=json_decode(stream_get_contents(STDIN),true,32,JSON_THROW_ON_ERROR);
 if(!is_array($in)) throw new Exception('输入必须是对象');

 $digest=md5($in['text']);$result=['lowercase'=>$digest,'uppercase'=>strtoupper($digest)];

 echo json_encode($result,JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_THROW_ON_ERROR);
} catch(Throwable $e) { fwrite(STDERR,$e->getMessage()); exit(1); }
