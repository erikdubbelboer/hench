
--[[
  Libraries can be included using 'require'
	For example:
	  require 'tools'
	
	After this the functions are usable from the global scope.
--]]

return {

	['getCookie'] = function(headers, name)
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
	end,

	['split'] = function(str, sep)
		local fields = {}
		local pattern = string.format("([^%s]+)", sep)
		string.gsub(str, pattern, function(c) fields[#fields+1] = c end)
		return fields
	end

}

