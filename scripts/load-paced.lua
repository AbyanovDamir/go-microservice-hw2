-- Optional supplementary test: roughly 833 requests/s at 500 connections.
-- wrk -t12 -c500 -d60s -s scripts/load-paced.lua http://localhost:8080/api/users
function delay() return 600 end
