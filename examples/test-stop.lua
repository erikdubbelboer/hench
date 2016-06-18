
local counter = 0

function request(state)
  return {
    ['method' ] = 'GET',
    ['url'    ] = 'http://127.0.0.1:9090/'
  }
end

function response(res, state)
	counter = counter + 1

	if counter == 30 then
		stop:close()
	end

  return true
end

