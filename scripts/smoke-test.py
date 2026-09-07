"""Integration tests against an isolated local instance; uses only examples and temporary credentials."""
import io
import json
import os
import time
import urllib.error
import urllib.request
import uuid
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BASE = os.environ.get("TOOLDECK_TEST_URL", "http://127.0.0.1:18088")
ENV = dict(line.split("=", 1) for line in (ROOT / ".env").read_text().splitlines() if "=" in line)
TOKEN = ""


def request(path, method="GET", data=None, token=None, headers=None, raw=False):
    headers = dict(headers or {})
    token = TOKEN if token is None else token
    if token:
        headers["Authorization"] = "Bearer " + token
    if isinstance(data, dict):
        data = json.dumps(data).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(BASE + "/api" + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=35) as r:
            body = r.read()
            return r.status, body if raw else json.loads(body)["data"]
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()


def upload(path, name, content, mode=""):
    boundary = uuid.uuid4().hex
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{name}"\r\nContent-Type: application/octet-stream\r\n\r\n').encode() + content + b"\r\n"
    if mode:
        body += (f'--{boundary}\r\nContent-Disposition: form-data; name="mode"\r\n\r\n{mode}\r\n').encode()
    body += f"--{boundary}--\r\n".encode()
    return request(path, "POST", body, headers={"Content-Type": "multipart/form-data; boundary=" + boundary})


def package(name, source=None, manifest=None):
    directory = ROOT / "examples" / name
    if manifest is None:
        manifest = json.loads((directory / "tooldeck.json").read_text(encoding="utf-8"))
    manifest["version"] = "test-" + uuid.uuid4().hex[:8]
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("tooldeck.json", json.dumps(manifest))
        if source is not None:
            z.writestr(manifest["entrypoint"], source)
        else:
            for p in directory.rglob("*"):
                if p.is_file() and p.name != "tooldeck.json":
                    z.write(p, p.relative_to(directory))
    status, result = upload("/v1/tools", name + ".zip", output.getvalue())
    assert status == 201, (status, result)
    return result


def execute(tool, inputs, expected="succeeded", token=None):
    status, run = request(f'/v1/tools/{tool["id"]}/runs', "POST", {"input": inputs}, token=token)
    assert status in (200, 202), (status, run)
    deadline = time.monotonic() + 100
    while run["status"] in ("queued", "running") and time.monotonic() < deadline:
        time.sleep(1)
        status, run = request("/v1/runs/" + run["run_id"], token=token)
        assert status == 200, (status, run)
    assert run["status"] == expected, run
    return run


def main():
    global TOKEN
    assert request("/v1/tools", token="")[0] == 401
    status, login = request("/core/login", "POST", {"username": "admin", "password": ENV["TOOLDECK_ADMIN_PASSWORD"]}, token="")
    assert status == 200, login
    TOKEN = login["access_token"]
    tools = {}
    for runtime in ("python", "php", "node", "go"):
        tool = package("echo-" + runtime)
        tools[runtime] = tool
        run = execute(tool, {"text": "你好 ToolDeck"})
        assert run["result"]["echo"] == "你好 ToolDeck", run
        print(runtime + ": real Docker execution passed", flush=True)
    image = package("image-form-demo")
    first = execute(image, {"prompt": "a blue morning", "reference_images": [], "aspect_ratio": "16:9"})
    artifact = first["artifacts"][0]
    status, png = request("/v1/files/" + artifact["file_id"], raw=True)
    assert status == 200 and png.startswith(b"\x89PNG")
    status, file = upload("/v1/files", "reference.png", png)
    assert status == 201, file
    second = execute(image, {"prompt": "with a reference", "reference_images": [file["file_id"]], "aspect_ratio": "1:1"})
    assert second["result"]["references"][0]["size"] == len(png)
    print("image upload, mounted reference, generated artifact and authorized download passed", flush=True)
    status, key = request("/v1/keys", "POST", {"name": "smoke-test", "tools": ["echo-python"], "days": 1})
    assert status == 201, key
    execute(tools["python"], {"text": "api key"}, token=key["key"])
    assert request("/v1/keys", token=key["key"])[0] == 403
    assert request("/v1/files/" + artifact["file_id"], token=key["key"], raw=True)[0] == 404
    assert request(f'/v1/tools/{image["id"]}/runs', "POST", {"input": {}}, token=key["key"])[0] == 404
    request("/v1/keys/" + key["id"], "DELETE")
    assert request("/v1/tools", token=key["key"])[0] == 401
    print("API key scopes, foreign file denial and revocation passed", flush=True)
    m = json.loads((ROOT / "examples/echo-python/tooldeck.json").read_text(encoding="utf-8"))
    m["name"] = "network-check"
    m["network"] = {"enabled": True, "allowed_hosts": ["example.com"]}
    source = '''import json, urllib.request, socket
with urllib.request.urlopen("https://example.com", timeout=15) as response:
 status=response.status
try:
 socket.create_connection(("1.1.1.1",443),timeout=2)
 direct=True
except OSError:
 direct=False
try:
 urllib.request.urlopen("http://127.0.0.1:8080",timeout=2)
 private=True
except OSError:
 private=False
print(json.dumps({"status":status,"direct":direct,"private":private}))
'''
    nettool = package("network-check", source=source, manifest=m)
    run = execute(nettool, {"text": "network"})
    assert run["result"] == {"status": 200, "direct": False, "private": False}, run
    print("third-party HTTPS proxy and direct/private network denial passed", flush=True)
    m["name"] = "secret-check"
    m["network"] = {"enabled": False, "allowed_hosts": []}
    m["secrets"] = ["SMOKE_SECRET"]
    secret = "smoke-" + uuid.uuid4().hex
    assert request("/v1/secrets", "POST", {"name": "SMOKE_SECRET", "value": secret, "tools": ["secret-check"]})[0] == 200
    secret_tool = package("secret-check", source='import os,json,sys\nprint(os.environ["SMOKE_SECRET"],file=sys.stderr)\nprint(json.dumps({"present":bool(os.environ.get("SMOKE_SECRET"))}))\n', manifest=m)
    run = execute(secret_tool, {"text": "secret"})
    assert run["result"]["present"] and secret not in run["logs"] and "[REDACTED]" in run["logs"], run
    request("/v1/secrets/SMOKE_SECRET", "DELETE")
    m["name"] = "cancel-check"
    m["secrets"] = []
    m["execution"]["mode"] = "async"
    sleeper = package("cancel-check", source='import time,json\ntime.sleep(25)\nprint(json.dumps({"done":True}))\n', manifest=m)
    _, run = request(f'/v1/tools/{sleeper["id"]}/runs', "POST", {"input": {"text": "cancel"}})
    for _ in range(10):
        _, run = request("/v1/runs/" + run["run_id"])
        if run["status"] == "running": break
        time.sleep(0.3)
    status, canceled = request("/v1/runs/" + run["run_id"] + "/cancel", "POST")
    assert status == 200 and canceled["status"] == "canceled"
    time.sleep(1)
    assert request("/v1/runs/" + run["run_id"])[1]["status"] == "canceled"
    print("secret grant/injection, log redaction and running-task cancellation passed", flush=True)
    print("ALL INTEGRATION CHECKS PASSED", flush=True)


if __name__ == "__main__":
    main()
