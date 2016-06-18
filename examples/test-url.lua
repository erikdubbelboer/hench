
-- See: https://github.com/cjoudrey/gluaurl
local url = require('url')

if #args == 0 then
	println('No url passed')
	exit()
end

u = url.parse(args[1])
u.host = '127.0.0.1:9090'
u = url.build(u)

function request(state)
  return {
    ['method' ] = 'GET',
    ['url'    ] = u
  }
end

function response(res, state)
  return res.status ~= 200
end

