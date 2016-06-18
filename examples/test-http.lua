
local http = require('http')
local tools = require('tools')

-- For more options see: https://github.com/cjoudrey/gluahttp
response, err = http.get('http://dubbelboer.com/hench-urls.txt')
if err ~= nil then
	println(err)
	stop:close()
end

urls = tools.split(response.body, '\n')

function request(state)
	local url = table.remove(urls, 1)

	println(url)

  return {
    ['method' ] = 'GET',
    ['url'    ] = url
  }
end

function response(res, state)
	if #urls == 0 then
		stop:close()
	end

  return res.status ~= 200
end

