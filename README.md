
hench
=====

The Http bENCHmark tool.


Installing (requires go 1.4 or newer):
```bash
go get github.com/atomx/hench
```

Running:
```bash
hench -rps=1 -script=example.lua
```

Usage:
```bash
Usage of hench:
  -cachedns
        Cache dns lookups (dns lookup time is included in the request time and might slow things down) (default true)
  -compression
        Enable or disable compression (default true)
  -keepalive
        Use keepalive connections (default true)
  -rps int
        The maximum number of requests per second (default 10)
  -script string
        Optional Lua script to run
  -workers int
        Number of workers to use (number of concurrent requests) (default 100)
```

