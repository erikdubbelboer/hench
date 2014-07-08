
local urls = {
  '/placement?id=1&type=iframe&size=300x250', -- normal bid
  '/placement?id=1&type=iframe&size=300x250', -- normal bid
  '/placement?id=1&type=iframe&size=300x250', -- normal bid
  '/placement?id=1&type=iframe&size=250x250', -- normal bid

  '/placement?id=1&type=iframe&size=120x600', -- psa

  '/placement?id=3&type=iframe&size=120x600', -- normal bid (psa after 1 view)
  '/placement?id=3&type=iframe&size=300x250', -- normal bid (psa after 2 views)

  '/placement?id=2&type=iframe&size=120x600', -- normal bid (unless segment is set)
  '/placement?id=2&type=iframe&size=300x250', -- psa (unless segment is set)
  '/segment?id=1', -- do more segment than unsegment
  '/segment?id=1',
  '/segment?id=1',
  '/unsegment?id=1'
}


function getCookie(cookie, name)
  local start = string.find(cookie, name .. '=')

  if start == nil then
    return nil
  end

  return string.sub(cookie, start + #name + 1, string.find(cookie, ';', start) - 1)
end


local uids    = {}
local maxuids = 2


function response(res, state)
  if state.newuid and res.headers['Set-Cookie'] then
    local uid = getCookie(res.headers['Set-Cookie'][1], 'uid')

    if uid ~= nil then
      if #uids < maxuids then
        uids[#uids + 1] = uid
      else
        -- Overwrite a random uid.
        uids[math.random(#uids)] = uid
      end
    end
  end
end


function request(state)
  local req = {
    ['method' ] = 'GET',
    ['url'    ] = 'http://london.spotmx.com:9080' .. urls[math.random(#urls)],
    ['headers'] = {}
  }

  -- Have at least maxuids uid's before we start reusing.
  if #uids >= maxuids and math.random(100) > 20 then -- 20% chance to be a new user.
    state.newuid = false

    req.headers['Cookie'] = 'uid=' .. uids[math.random(#uids)]
  else
    state.newuid = true
  end

  return req
end

