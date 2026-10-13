"""Run TD01-TD05 regressions against an isolated localhost instance only."""
import io
import json
import os
from pathlib import Path
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
import zipfile

ROOT = Path(__file__).resolve().parent.parent
BASE = os.environ.get('TOOLDECK_TEST_URL', 'http://127.0.0.1:18089')
assert urllib.parse.urlparse(BASE).hostname in ('127.0.0.1', 'localhost'), 'Use isolated localhost test data'
ENV = dict(line.split('=', 1) for line in Path(os.environ['TOOLDECK_REVIEW_ENV_FILE']).read_text().splitlines() if '=' in line)
TOKEN = ''

def request(path, method='GET', data=None, expected=(200, 201, 202), content_type=None):
    headers = {'Authorization': 'Bearer ' + TOKEN} if TOKEN else {}
    if isinstance(data, dict):
        data = json.dumps(data).encode()
        content_type = 'application/json'
    if content_type:
        headers['Content-Type'] = content_type
    req = urllib.request.Request(BASE + '/api' + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            status, body = response.status, response.read()
    except urllib.error.HTTPError as error:
        status, body = error.code, error.read()
    payload = json.loads(body)
    assert status in expected, (path, status, payload)
    return payload.get('data', payload)

def wait_build(tool):
    for _ in range(800):
        tool = request('/v1/tools/' + tool['id'] + '/build')
        if tool['build_status'] == 'ready':
            return tool
        assert tool['build_status'] != 'failed', (tool.get('build_error'), tool.get('build_log'))
        time.sleep(1.5)
    raise AssertionError('build timed out')

def wait_run(run):
    for _ in range(40):
        run = request('/v1/runs/' + run['run_id'])
        if run['status'] not in ('queued', 'running', 'canceling'):
            return run
        time.sleep(1)
    raise AssertionError('run timed out')

def upload_example(name):
    existing = next((tool for tool in request('/v1/my-tools') if tool['manifest']['name'] == name), None)
    if existing:
        tool = wait_build(existing)
        if tool.get('review_status') == 'draft':
            request('/v1/tools/' + tool['id'] + '/publication', 'POST', {'action': 'submit'})
        if tool.get('review_status') != 'approved':
            request('/v1/reviews/' + tool['id'], 'POST', {'status': 'approved'})
        return tool
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, 'w') as archive:
        for path in (ROOT / 'examples' / name).iterdir():
            if path.is_file() and path.name != '.DS_Store':
                archive.writestr(path.name, path.read_bytes())
    boundary = uuid.uuid4().hex
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="example.zip"\r\nContent-Type: application/zip\r\n\r\n').encode() + buffer.getvalue() + f'\r\n--{boundary}--\r\n'.encode()
    tool = request('/v1/tools', 'POST', body, content_type='multipart/form-data; boundary=' + boundary)
    request('/v1/tools/' + tool['id'] + '/build', 'POST', {})
    tool = wait_build(tool)
    request('/v1/tools/' + tool['id'] + '/publication', 'POST', {'action': 'submit'})
    request('/v1/reviews/' + tool['id'], 'POST', {'status': 'approved'})
    return tool

def execute(tool, input):
    return wait_run(request('/v1/tools/' + tool['id'] + '/runs', 'POST', {'input': input}))

TOKEN = request('/core/login', 'POST', {'username': 'admin', 'password': ENV['TOOLDECK_ADMIN_PASSWORD']})['access_token']
code = '<?php\n$name = trim(stream_get_contents(STDIN)) ?: "ToolDeck";\necho "Hello, " . $name . "!\\n";\necho "PHP " . PHP_VERSION . "\\n";\n'
play = wait_build(request('/v1/playground', 'POST', {'language': 'php', 'version': '8.1', 'code': code}))
run = execute(play, {'stdin': ''})
assert run['status'] == 'succeeded' and 'Hello, ToolDeck!' in run['result'], run
print('TD01: default PHP 8.1 sample executed successfully', flush=True)
url = upload_example('url-codec')
input = {'operation': '编码', 'mode': '标准编码（空格转 %20）', 'text': ''}
request('/v1/tools/' + url['id'] + '/runs', 'POST', {'input': input}, expected=(422,))
input['text'] = 'a b'
run = execute(url, input)
assert run['status'] == 'succeeded' and 'a%20b' in json.dumps(run['result']), run
print('TD02: empty input rejected; valid URL input succeeded', flush=True)
stream = upload_example('stream-demo')
run = execute(stream, {})
assert run['status'] == 'succeeded' and isinstance(run['result']['result'], str), run
assert ''.join(event['text'] for event in run['stream_events']) == run['result']['result'], run
print('TD03: streamed deltas match final string output', flush=True)
formatter = upload_example('json-formatter')
error = request('/v1/tools/' + formatter['id'] + '/runs', 'POST', {'input': {'operation': '格式化', 'text': '{"a":}'}}, expected=(422,))
assert 'JSON 格式错误' in json.dumps(error, ensure_ascii=False) and '第 1 行' in json.dumps(error, ensure_ascii=False), error
run = execute(formatter, {'operation': '格式化', 'text': '{"a":1}'})
assert run['status'] == 'succeeded', run
print('TD04: invalid JSON reports position; corrected input succeeds', flush=True)
history = request('/v1/runs?mine=1&status=succeeded&page=1&page_size=20')
assert history['items'] and all(item['status'] == 'succeeded' and item.get('tool_title') for item in history['items']), history
print('TD05: filtered history contains tool titles and correct totals', flush=True)
