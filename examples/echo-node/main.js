const fs = require("node:fs"); const input=JSON.parse(fs.readFileSync(0,"utf8")); console.log(JSON.stringify({echo:input.text,runtime:"node"}));
