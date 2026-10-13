"""Build the three reviewed example packages without local files or credentials."""
import hashlib
import json
from pathlib import Path
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / 'work' / 'releases' / 'site-review-20261013'
OUT.mkdir(parents=True, exist_ok=True)
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
packages = []
for name in ('url-codec', 'json-formatter', 'stream-demo'):
    source = ROOT / 'examples' / name
    manifest = json.loads((source / 'tooldeck.json').read_text())
    package = OUT / f'{name}-{manifest["version"]}.zip'
    with zipfile.ZipFile(package, 'w', zipfile.ZIP_DEFLATED) as archive:
        for filename in ('tooldeck.json', 'main.php'):
            archive.write(source / filename, filename)
    with zipfile.ZipFile(package) as archive:
        assert archive.testzip() is None
        assert set(archive.namelist()) == {'tooldeck.json', 'main.php'}
    packages.append({'file': package.name, 'version': manifest['version'], 'sha256': hashlib.sha256(package.read_bytes()).hexdigest()})
(OUT / 'packages.json').write_text(json.dumps({'source_revision': revision, 'packages': packages}, ensure_ascii=False, indent=2) + '\n')
print(OUT)
