
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

Usage:
```bash
$ ./hench -h
Usage of ./hench:
  -keepalive=true: Use keepalive connections.
  -rps=10: Maximum number of requests per second.
  -script="simple.lua": The lua script to run.
  -workers=100: Number of workers to use (number of concurrent requests).
```


Pushing it to the max
---------------------

```bash
$ ulimit -n 100000
$ GOMAXPROCS=`cat /proc/cpuinfo | grep processor | wc -l` ./hench -rps=10000 -workers=200 -script=simple.lua
```

You might need to run this as root because of the ulimit.

You might also need to increase `-rps` if it's already reaching the 10000.

Now increase `-workers` until the number of requests per second doesn't go any higher.

