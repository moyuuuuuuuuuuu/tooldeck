"""Create uploadable ZIPs without including parent directories."""
from pathlib import Path
from zipfile import ZipFile, ZIP_DEFLATED

root = Path(__file__).resolve().parent.parent
output = root / "work" / "examples"
output.mkdir(parents=True, exist_ok=True)
downloads = root / "web" / "public" / "tool-templates"
downloads.mkdir(parents=True, exist_ok=True)
blank_templates = {"blank-php", "blank-js", "blank-node", "blank-python", "blank-go"}
for source in (root / "examples").iterdir():
    if not source.is_dir():
        continue
    targets = [output / (source.name + ".zip")]
    if source.name in blank_templates:
        targets.append(downloads / (source.name + ".zip"))
    for target in targets:
        with ZipFile(target, "w", ZIP_DEFLATED) as archive:
            for file in source.rglob("*"):
                relative = file.relative_to(source)
                if file.is_file() and not {"vendor", "node_modules", "__pycache__"}.intersection(relative.parts):
                    archive.write(file, relative)
print(output)
