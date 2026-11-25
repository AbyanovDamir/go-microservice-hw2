-- A declared arrival profile for the 60-second test, not a saturation test.
-- 500 open connections, independent uniform pacing; expected ~1030-1060 req/s.
-- Independent deterministic seeds avoid synchronizing all 12 wrk threads.
local next_id = 0

function setup(thread)
    next_id = next_id + 1
    thread:set("thread_id", next_id)
end

function init(args)
    math.randomseed(8123 + thread_id * 997)
end

function delay()
    return math.random(100, 830)
end
