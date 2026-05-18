# Python Plugins

This directory contains the source Python plugins embedded into the Atlas-Recon Go binary.

`./build.sh` copies these files into `internal/plugins/python/` before compiling so the binary can extract and run them from the operating system's user cache directory.

## Dependency Handling

The Go CLI creates a managed Python virtual environment for plugin package dependencies. For direct plugin development, install the dependencies needed by the plugin you are editing into your own virtual environment.

Example for DNS lookup development:

```bash
python -m venv .venv
. .venv/bin/activate
python -m pip install dnspython
python plugins/python/dns_lookup.py example.com --types A,MX,TXT --json
```

## Direct Usage Examples

```bash
python plugins/python/dns_lookup.py example.com --types A,MX,TXT --json
python plugins/python/dns_lookup.py --file domains.txt --types A,AAAA,NS --csv
python plugins/python/dns_lookup.py 8.8.8.8 --types PTR
```

Prefer invoking plugins through `./ct` for normal use so configuration, dependency setup, result storage, and JSON handling are consistent.
