
function getCookie(headers, name)
  name = name .. '='

  for header,values in pairs(headers) do
    if header == 'set-cookie' then
      for _,value in ipairs(values) do
        local start = string.find(value, name)

        if start ~= nil then
          local stop = string.find(value, ';', start)

          if stop == nil then
            return string.sub(value, start + #name)
          else
            return string.sub(value, start + #name, stop - 1)
          end
        end
      end
    end
  end

  return nil
end


