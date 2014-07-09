
local counter = 0


function request(state)
  counter = counter + 1
  
  if counter % 1000 == 0 then
    print(counter)
  end

  state.counter = counter

  return {
    ['method' ] = 'GET',
    ['url'    ] = 'http://127.0.0.1:9090/test',
    ['headers'] = {
      ['X-Test'] = counter
    }
  }
end


function response(res, state)
  if res.status ~= 200 then
    print('expected res.status to be 200 not ' .. res.status)
  end
  if res.body ~= tostring(state.counter) then
    print('expected res.body to be ' .. state.counter .. ' not ' .. res.body)
  end

  return true
end

