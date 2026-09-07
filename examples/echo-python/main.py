import json,sys
print(json.dumps({"echo":json.load(sys.stdin)["text"],"runtime":"python"}))
