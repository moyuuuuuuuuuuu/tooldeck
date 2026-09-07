"""Create uploadable ZIPs without including parent directories."""
from pathlib import Path
from zipfile import ZipFile, ZIP_DEFLATED

root = Path(__file__).resolve().parent.parent
output = root / "work" / "examples"
output.mkdir(parents=True, exist_ok=True)
for source in (root / "examples").iterdir():
    if not source.is_dir():
        continue
    with ZipFile(output / (source.name + ".zip"), "w", ZIP_DEFLATED) as archive:
        for file in source.rglob("*"):
            if file.is_file():
                archive.write(file, file.relative_to(source))
print(output)
