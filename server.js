
var cluster = require('cluster');
var http    = require('http');

var workers = 1;

if (cluster.isMaster) {
  for (var i = 0; i < workers; ++i) {
    cluster.fork();
  }
} else {
  http.createServer(function (req, res) {
    if (req.url == '/test') {
      res.writeHead(200, {
        'Set-Cookie': 'foo=bar'
      });
      res.end(req.headers['x-test'])
    } else {
      console.log(req.method + ' ' + req.url)
      console.log(req.headers)

      res.writeHead(200, {
        'X-Foo': ['bar', 'baz'],
      });
      res.end('test');
    }
  }).listen(9090, '127.0.0.1');
}

