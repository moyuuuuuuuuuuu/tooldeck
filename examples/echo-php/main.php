<?php
$input=json_decode(stream_get_contents(STDIN),true,512,JSON_THROW_ON_ERROR); echo json_encode(["echo"=>$input["text"],"runtime"=>"php"],JSON_THROW_ON_ERROR);
