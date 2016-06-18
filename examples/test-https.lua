
function request(state)
  return {
    ['method' ] = 'GET',
    ['url'    ] = 'https://www.google.com'
  }
end

function response(res, state)
  return res.status == 200
end

