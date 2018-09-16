***RedisWork***

Recently, I was working on some redis instances and wanted to perform some actions on them. It became very difficult to perform those actions, as I could not find tools that allowed those actions. So I thought it might a good change to come up with something.

In my free time, I was learning GoLang, so this became another motivation for me to write in Go.

I am fairly new in GoLang :) and this is the best I can come up with.

---

****Build****

- This will install `rediswork` in your system and hopefully it will work

`go install github.com/mudasirmirza/rediswork`

- To fetch source of the tool

`go get github.com/mudasirmirza/rediswork`

---

****Usage****

- populate redis with random data

`rediswork -srcRedisDB=5 -srcRedisHost=127.0.0.1:6377 -populateData=true -populateCount=50000`

- copy keys from source to destination redis

`rediswork -copyKeys=true -srcRedisHost=localhost:6379 -srcRedisDB=1 -dstRedisHost=localhost:6377 -dstRedisDB=1 -scanCount=1000 -copyKeyCount=1000`

- check old keys which have not been touched for certain time (using `object idletime` command), time in seconds

`rediswork -checkOldKey=true -srcRedisHost=localhost:6379 -srcRedisDB=1 -keyAge=100`

- Print scanned keys

`rediswork -srcRedisDB=5 -srcRedisHost=127.0.0.1:6377 -scanCount=1000 -printKeys=true`

- Delete old keys which have not been touched for certain time (using `object idletime` command), time in seconds

`rediswork -checkOldKey=true -srcRedisHost=localhost:6379 -srcRedisDB=1 -keyAge=100 -deleteKeys=true`

- Delete old connection to redis server, redis server usually never kills client connections, so this can be handy

`rediswork -checkConnAge=true -srcRedisHost=localhost:6379 -delOldConnAge=2 -delOldConnIdle=2 -delOldConn=true`
