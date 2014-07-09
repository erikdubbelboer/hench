
hench
=====

The Http bENCHmark tool.


Installing (requires go 1.2 or newer):
```bash
$ GOPATH=`pwd` go get github.com/spotmx/hench
$ sudo cp bin/hench /usr/local/bin/
```

Running:
```bash
$ ./hench -rps=1 -script=simple.lua
```

Running for high volume:
```bash
$ GOMAXPROCS=6 ./hench -rps=1000 -script=simple.lua
```

Usage:
```bash
$ ./hench -h
Usage of ./hench:
  -keepalive=true: Use keepalive connections.
  -rps=10: Maximum number of requests per second.
  -script="simple.lua": The lua script to run.
  -workers=100: Number of workers to use (number of concurrent requests).
```

