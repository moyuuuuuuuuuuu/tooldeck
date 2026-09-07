<?php
$text='';
foreach (['你好，','这是 ','ToolDeck ','实时输出。'] as $chunk) {
 $text.=$chunk;
 fwrite(STDERR,'TOOLDECK_EVENT '.json_encode(['type'=>'delta','text'=>$chunk],JSON_UNESCAPED_UNICODE)."\n");
 fflush(STDERR);sleep(1);
}
echo json_encode(['text'=>$text],JSON_UNESCAPED_UNICODE);
