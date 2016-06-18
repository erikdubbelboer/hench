
local tools = require('tools')

local counter = 0
local workers = 0

function worker(state)
	state.worker = workers
	workers = workers + 1

	state.counter = 0
end

function request(state)
  counter = counter + 1
  
  if counter % 1000 == 0 then
    println(counter)
  end

  state.counter = counter

  return {
    ['method' ] = 'GET',
    ['url'    ] = 'http://127.0.0.1:9090/test',
    ['headers'] = {
      ['X-Test']   = counter,
			['X-Worker'] = state.worker
    }
  }
end

function response(res, state)
  if res.status ~= 200 then
    println('expected res.status to be 200 not ' .. res.status)
  end
  if res.body ~= tostring(state.counter) then
    println('expected res.body to be ' .. state.counter .. ' not ' .. res.body)
  end

  local foo = tools.getCookie(res.headers, 'foo')
  if foo ~= 'bar' then
    println('expected foo cookie to be "bar" not ' .. tostring(foo))
  end

  return true
end

