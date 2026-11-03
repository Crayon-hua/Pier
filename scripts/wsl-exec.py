"""Go -exec adapter for running cross-compiled Linux tests on Windows/WSL.

From server/ in PowerShell:
  $env:GOOS='linux'; $env:GOARCH='amd64'
  go test -exec 'python /absolute/path/to/pier/scripts/wsl-exec.py' ./internal/...
"""
import re
import subprocess
import sys

def linux_path(value):
    return re.sub(r'([A-Za-z]):[\\/]([^\r\n]*)', lambda m: '/mnt/' + m.group(1).lower() + '/' + m.group(2).replace('\\', '/'), value)

sys.exit(subprocess.call(['wsl', '-d', 'Ubuntu', '--'] + [linux_path(arg) for arg in sys.argv[1:]]))
