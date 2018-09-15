
- copy keys from source to destination redis

`./main -copyKeys=true -srcRedisHost=localhost:6379 -srcRedisDB=1 -dstRedisHost=localhost:6377 -dstRedisDB=1 -scanCount=1000 -copyKeyCount=1000`

- check old keys (using object idletime)

`./main -checkOldKey=true -srcRedisHost=localhost:6379 -srcRedisDB=1 -keyAge=100`

- delete old keys (using object idletime)

`./main -checkOldKey=true -srcRedisHost=localhost:6379 -srcRedisDB=1 -keyAge=100 -deleteKeys=true`

- Delete old connection to redis server

`./main --checkConnAge=true -srcRedisHost=localhost:6379 -delOldConnAge=2 -delOldConnIdle=2 -delOldConn=true`