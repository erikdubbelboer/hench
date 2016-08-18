
local http = require('http')
local tools = require('tools')

-- For more options see: https://github.com/cjoudrey/gluahttp
local res, err = http.get('http://dubbelboer.com/hench-urls.txt')
if err ~= nil then
	println(err)
	stop()
end

local urls = tools.split(res.body, '\n')
local wait = #urls

function request(state)
	if #urls == 0 then
		return -- Do nothing.
	end

	local url = table.remove(urls, 1)

	println(url)

  return {
    ['method' ] = 'GET',
    ['url'    ] = url
  }
end

function response(res, state)
	-- Only stop when all responses are received.
	wait = wait - 1
	if wait == 0 then
		stop()
	end

  return res.status == 200
end

