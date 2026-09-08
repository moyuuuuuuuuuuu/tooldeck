export function apiExamples(base: string, toolId: string, input: unknown, env?: Record<string, string>, asynchronous = false) {
  const url = `${base}/api/v1/tools/${toolId}/runs`
  const payload: Record<string, unknown> = env && Object.keys(env).length ? { input, env } : { input }
  if (asynchronous) {
    payload.callback_url = 'https://example.com/tooldeck/callback'
    payload.callback_secret = 'YOUR_CALLBACK_SECRET'
  }
  const body = JSON.stringify(payload)
  const quote = JSON.stringify
  const php = (value: string) => "'" + value.replace(/\\/g, '\\\\').replace(/'/g, "\\'") + "'"
  const note = asynchronous ? '最终结果将由 ToolDeck POST 到 callback_url。' : '同步工具直接返回最终结果。'
  return {
    PHP: `<?php
$ch = curl_init(${php(url)});
curl_setopt_array($ch, [CURLOPT_POST => true, CURLOPT_RETURNTRANSFER => true,
  CURLOPT_HTTPHEADER => ['X-API-Key: ' . getenv('TOOLDECK_API_KEY'), 'Content-Type: application/json'],
  CURLOPT_POSTFIELDS => ${php(body)}]);
$response = curl_exec($ch);
if ($response === false) throw new Exception(curl_error($ch));
echo $response;
// ${note}`,
    Java: `// Java 11+
HttpRequest request = HttpRequest.newBuilder(URI.create(${quote(url)}))
  .header("X-API-Key", System.getenv("TOOLDECK_API_KEY"))
  .header("Content-Type", "application/json")
  .POST(HttpRequest.BodyPublishers.ofString(${quote(body)})).build();
System.out.println(HttpClient.newHttpClient().send(request, HttpResponse.BodyHandlers.ofString()).body());
// ${note}`,
    Python: `import json, os, urllib.request
headers = {"X-API-Key": os.environ["TOOLDECK_API_KEY"], "Content-Type": "application/json"}
request = urllib.request.Request(${quote(url)}, data=${quote(body)}.encode(), headers=headers, method="POST")
with urllib.request.urlopen(request, timeout=30) as response:
    print(json.dumps(json.load(response), ensure_ascii=False, indent=2))
# ${note}`,
    Go: `req, _ := http.NewRequest("POST", ${quote(url)}, strings.NewReader(${quote(body)}))
req.Header.Set("X-API-Key", os.Getenv("TOOLDECK_API_KEY"))
req.Header.Set("Content-Type", "application/json")
resp, err := (&http.Client{Timeout: 30*time.Second}).Do(req)
if err != nil { panic(err) }
defer resp.Body.Close()
body, _ := io.ReadAll(resp.Body)
fmt.Println(string(body))
// ${note}`
  }
}
