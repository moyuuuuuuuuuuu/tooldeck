import hashlib,json,os,struct,sys,zlib
params=json.load(sys.stdin)
seed=hashlib.sha256(params['prompt'].encode()).digest()
w,h={'1:1':(512,512),'16:9':(640,360),'9:16':(360,640)}[params['aspect_ratio']]
raw=b''.join(b'\0'+bytes(c for x in range(w) for c in ((seed[0]+x//3)%256,(seed[1]+y//3)%256,(seed[2]+(x+y)//5)%256)) for y in range(h))
def chunk(t,b):return struct.pack('!I',len(b))+t+b+struct.pack('!I',zlib.crc32(t+b)&0xffffffff)
png=b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('!2I5B',w,h,8,2,0,0,0))+chunk(b'IDAT',zlib.compress(raw))+chunk(b'IEND',b'')
with open(os.path.join(os.environ['TOOLDECK_OUTPUT_DIR'],'preview.png'),'wb') as f:f.write(png)
references=[{'size':os.path.getsize(p)} for p in params.get('reference_images',[])]
print(json.dumps({'message':'本地图片表单示例，不是 AI 生成结果','prompt':params['prompt'],'references':references},ensure_ascii=False))
