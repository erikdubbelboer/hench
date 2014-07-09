
--[[
  The script itself is only executed once.

  You can use global variables which will be shared between all requests.
]]--

local counter = 0


--[[
  The request function is called each time a request is constructed.

  Arguments:
    0: An object that can be used to keep a state between the request and response function.

  Return value:
    An object containing the request that should be performed.
]]--
function request(state)
  counter = counter + 1

  return {
    ['method' ] = 'GET',
    ['url'    ] = 'http://127.0.0.1:9090/',
    ['headers'] = {
      ['User-Agent'] = 'simple hench',
      ['X-Foo']      = counter
    }
  }
end


--[[
  The response function is called each time after a response is returned.

  Arguments:
    0: An object containing the response.
       For example:
         {
           ['status']  = 200,
           ['body']    = 'test',
           ['headers'] = {
             ['X-Foo'] = {
               'bar',
               'baz'
             },
             ['Connection'] = 'keep-alive'
           }
         }
    1: An object that can be used to keep a state between the request and response function.

  Return value:
    Weather the request was a success or not. Returning anything other than true will increase the error counter.
]]--
function response(res, state)
  return res.status == 200
end

