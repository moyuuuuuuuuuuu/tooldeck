$ErrorActionPreference = 'Stop'
Push-Location (Split-Path $PSScriptRoot)
try {
  $matrix = @{
    php = @('8.0','8.1','8.2','8.3')
    node = @('20','21','22','23')
    python = @('3.10','3.11','3.12')
    go = @('1.22','1.23','1.24')
  }
  foreach ($runtime in @('php','node','python','go')) {
    foreach ($version in $matrix[$runtime]) {
      $base = switch ($runtime) {
        php { if ($version -in @('8.0','8.1')) { "php:${version}-cli-bullseye" } else { "php:${version}-cli-bookworm" } }
        node { "node:${version}-bookworm-slim" }
        python { "python:${version}-slim-bookworm" }
        go { "golang:${version}-bookworm" }
      }
      docker build --build-arg "BASE_IMAGE=$base" -t "tooldeck-runtime-${runtime}:${version}-build2" -f "runtimes/${runtime}.Dockerfile" .
      if ($LASTEXITCODE -ne 0) { throw "Runtime build failed: ${runtime} ${version}" }
    }
  }
} finally { Pop-Location }
