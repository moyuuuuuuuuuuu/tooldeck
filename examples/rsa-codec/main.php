<?php
try {
 $in=json_decode(stream_get_contents(STDIN),true,32,JSON_THROW_ON_ERROR);
 if(!is_array($in)) throw new Exception('输入必须是对象');

 if($in['operation']==='加密') {
  $key=openssl_pkey_get_public($in['public_key']??'');
  if(!$key)throw new Exception('请输入有效的 PEM 公钥');
 }elseif($in['operation']==='解密') {
  $pem=getenv('RSA_PRIVATE_KEY');
  if(!$pem)throw new Exception('请在环境变量配置 RSA_PRIVATE_KEY');
  $key=openssl_pkey_get_private($pem,getenv('RSA_PASSPHRASE')?:'');
  if(!$key)throw new Exception('私钥或私钥口令无效');
 }else throw new Exception('无效操作');
 $details=openssl_pkey_get_details($key);
 if($details['type']!==OPENSSL_KEYTYPE_RSA||$details['bits']<2048||$details['bits']>4096)throw new Exception('仅支持 2048–4096 位 RSA 密钥');
 $size=intdiv($details['bits']+7,8);
 if($in['operation']==='加密') {
  if(strlen($in['text'])>$size-42)throw new Exception('文本过长，当前密钥最多支持 '.($size-42).' 字节');
  if(!openssl_public_encrypt($in['text'],$output,$key,OPENSSL_PKCS1_OAEP_PADDING))throw new Exception('RSA 加密失败');
  $result=['result'=>base64_encode($output),'padding'=>'OAEP-SHA1'];
 }else {
  $data=base64_decode($in['text'],true);
  if($data===false||strlen($data)!==$size||!openssl_private_decrypt($data,$output,$key,OPENSSL_PKCS1_OAEP_PADDING))throw new Exception('解密失败，请检查密文、密钥与填充算法');
  if(!preg_match('//u',$output))throw new Exception('解密结果不是 UTF-8 文本');
  $result=['result'=>$output];
 }

 echo json_encode($result,JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_THROW_ON_ERROR);
} catch(Throwable $e) { fwrite(STDERR,$e->getMessage()); exit(1); }
