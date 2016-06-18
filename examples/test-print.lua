
function request(state)
  return {
    ['method' ] = 'GET',
    ['url'    ] = 'http://127.0.0.1:9090/'
  }
end

function response(res, state)
	print('body: ')
	println(res.body)

  return true
end

