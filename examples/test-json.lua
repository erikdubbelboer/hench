
local json = require('json')

function request(state)
  return {
    ['method' ] = 'POST',
		['body'   ] = json.encode({
			['foo'] = 'bar'
		}),
    ['url'    ] = 'http://127.0.0.1:9090/json',
  }
end

function response(res, state)
	local d = json.decode(res.body)

	println(d.bar)

  return res.status == 200
end

