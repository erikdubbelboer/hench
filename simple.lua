
function response(res, state)
  return true
end


function request(state)
  return {
    ['method' ] = 'GET',
    ['url'    ] = 'http://london.spotmx.com:9080/',
    ['headers'] = {}
  }
end

