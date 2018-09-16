package main

import (
	"flag"
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/go-redis/redis"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"time"
)

// this is -1ms to check ttl value before restoring
// restore doesnt take -1 as ttl value
var ttl = 0 * time.Millisecond

// counter for number of keys migrated
var globalCounter int64

// Func to convert seconds to number of days
func secondsToDays(inSeconds int64) int64 {
	return int64(inSeconds / 60 / 60 / 24)
}

// Func to convert number of days to seconds (just in case)
func daysToSeconds(inDays int64) int64 {
	return int64(inDays * 24 * 60 * 60 )
}

// Func to check err, if error then panic
func check(err error) {
	if err != nil {
		panic(err)
	}
}

// Func to generate random string
func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

// Func to populate redis with random key->value
func populateRedis(c *redis.Client, n int) {
	pipe := c.Pipeline()
	start := time.Now()
	for i := 0; i <= n; i++ {
		pipe.Set(RandomString(11), RandomString(21), 0)
	}
	_, err := pipe.Exec()
	if err != nil {
		fmt.Println(err)
	}
	elapsed := time.Since(start)
	fmt.Printf("Created %d keys in %v\n", n, elapsed)
	os.Exit(0)
}

// Func to scan key(s) in redis
func scanKeys(c *redis.Client, ch chan<- []string, i int64, match string) {

	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = c.Scan(cursor, match, i).Result()
		check(err)
		ch <- keys
		if cursor == 0 {
			break
		}
	}
}

// Func to print scanned key(s)
func printScanKey(ch <-chan []string) {
	for {
		for _, v := range <-ch {
			fmt.Println(v)
		}
	}
}

// Func to get idle time info on key(s)
func keyInfo(c *redis.Client, inputCh <-chan []string, outputCh chan<- string, age float64, delTrue bool) {

	for {
		var keys []string
		keys = <- inputCh
		for _, v := range keys {
			globalCounter++
			t, err := c.ObjectIdleTime(v).Result()
			check(err)
			if t.Seconds() >= age {
				if delTrue {
					outputCh <- v
				} else {
					fmt.Println(v, t.Seconds())
				}
			}
		}
	}
}

// Func to delete a bunch of keys together using Pipeline
// Takes in
// c = redis.Client, object
// ch = Channel, which has keys being populated by other keyInfo goroutine
// n = int64, run pipe.Exec() after n number of keys are in pipeline
func delKeys(c *redis.Client, ch <-chan string, n int64, b bool) {

	pipe := c.Pipeline()
	var i int64 = 0
	for {
			if b {
				pipe.Del(<-ch)
			}
			if i >= n {
				_, err := pipe.Exec()
				check(err)
				fmt.Println("Deleted ", i, " keys")
				i = 0
			}
		i++
	}
}

// Func to migrate keys from source to destination
func copyKey(s *redis.Client, d *redis.Client, ch <-chan []string, t time.Time, n int64) {
	for {
		pd := d.Pipeline()
		i := int64(0)

		for _, v := range <-ch {

			// init a value to hold the key data
			var data string
			//var ttl time.Duration = 0 * time.Millisecond
			
			data = s.Dump(v).Val()
			// TODO: optimize copying TTL, currently taking allot of time and resources
			//ttl = s.PTTL(v).Val()
			// ttl -1ms is not allowed in restore
			//if ttl == td {
				//ttl = time.Millisecond * 0
			//}

			// restore on destination
			pd.Restore(v, ttl, data)

			if i >= n {
				_, err := pd.Exec()
				// when restoring, if key already present it gives error
				// ERR Target key name is busy, so this is a hack around it until I learn how to compare errors
				if err == nil || len(err.Error()) != 28 {
					check(err)
				}

				if err == nil {
					// increment global key counter
					globalCounter = globalCounter + i
					tm.Flush()
					tm.Clear()
					tm.Printf("\x0cRestored %d keys in %v time", globalCounter, time.Since(t))
				}
			}
			i++
		}
	}
}

// Func to check client connection details
func clientConnDetails(s *redis.Client) []string {
	var clientList string
	clientList, _ = s.ClientList().Result()

	// Could not find any other thing to perform SPLIT on the conn detail line with regex
	re := regexp.MustCompile(`id=[0-9]+`)
	result := re.Split(clientList, -1)

	// Don't know why the first element in slice is nil, so this is to remove the first element
	if len(result[0]) == 0 {
		copy(result[0:], result[0+1:])
		result[len(result)-1] = ""
		result = result[:len(result)-1]
	}

	return result
}

// Func to parse client connection details and return []string of addr:port after checking the age and idle time with provided arguments
func parseClientConn(s []string, connAge int64, connIdle int64) []string {
	//panic("Not implemented")
	pattern := `addr=(?P<addr>.*) fd=.* age=(?P<age>.*) idle=(?P<idle>.*) flags=.*`
	patternMetadata := regexp.MustCompile(pattern)
	var connectionList []string
	for _, v := range s {
		var ageCheck = false
		var idleCheck = false
		var remoteAddr string
		match := patternMetadata.FindStringSubmatch(v)
		result := make(map[string]string)
		for i, name := range patternMetadata.SubexpNames() {
			if i != 0 && name != "" {
				result[name] = match[i]
			}
		}
		for k, v := range result {
			if k == "age" {
				x, _ := strconv.ParseInt(v, 10, 64)
				if secondsToDays(x) > connAge {
					ageCheck = true
				}
			} else if k == "idle" {
				x, _ := strconv.ParseInt(v, 10, 64)
				if secondsToDays(x) > connIdle {
					idleCheck = true
				}
			} else if k == "addr" {
				remoteAddr = v
			}
		}
		if ageCheck == true || idleCheck == true {
			connectionList = append(connectionList, remoteAddr)
		}
	}
	return connectionList
}

// Func to delete client connections older than provided arguments
func deleteClientConn(s *redis.Client, c []string){
	for _, v := range c {
		res := s.ClientKill(v)
		fmt.Println("Connection deleted: ", res)
	}
}

// Func to create redis connection to source redis and returns redis client
func connectSrcRedis(r string, p int) *redis.Client {
	return redis.NewClient(&redis.Options {
		Addr: r,
		Password: "",
		DB: p,
		MaxRetries: 5,
		ReadTimeout: 5 * time.Minute,
		IdleTimeout: 5 * time.Minute,
		MinIdleConns: 5,
		PoolSize: 100,
	})
}

// Func to create redis connection to destination redis and returns redis client
func connectDstRedis(r string, p int) *redis.Client {
	return redis.NewClient(&redis.Options {
		Addr: r,
		Password: "",
		DB: p,
		MaxRetries: 5,
		ReadTimeout: 5 * time.Minute,
		IdleTimeout: 5 * time.Minute,
		MinIdleConns: 5,
		PoolSize: 100,
	})
}

func main() {
	// action flags
	copyKeys := flag.Bool("copyKeys", false, "Should we migrate keys from source to destination, default: false")
	checkOldKey := flag.Bool("checkOldKey", false, "Check how many keys are older then provided keyAge, default: false")
	deleteKeys := flag.Bool("deleteKeys", false, "Should we delete keys or no, default: false")
	printKeys := flag.Bool("printKeys", false, "Only print the scanned keys, default: true")
	populateData := flag.Bool("populateData", false, "Populate redis with random key->value, default: false")
	checkConnAge := flag.Bool("checkConnAge", false, "Check age of connections created by client, default: false")

	//redis connection info flags
	srcRedisHost := flag.String("srcRedisHost", "localhost:6379", "Source Redis Host, default: localhost:6379")
	srcRedisDB := flag.Int("srcRedisDB", 1, "Redis DB, default 1")
	dstRedisHost := flag.String("dstRedisHost", "localhost:6377", "Destination Redis Host, default: localhost:6377")
	dstRedisDB := flag.Int("dstRedisDB", 1, "Redis DB, default 1")

	// old key check info flags
	keyAge := flag.Float64("keyAge", 5184000, "Key age defined in seconds, key should not be used within this time, only used with deleteKeys, default: 5184000 (60 days in seconds)")
	keyMatch := flag.String("keyMatch", "*", "Prefix to match in scan, default *")

	// populate redis key count
	populateCount := flag.Int("populateCount", 50000, "How many random keys to populate, default: 50000")

	// misc flags used in copyKeys
	scanCount := flag.Int64("scanCount", 10000, "Scan count, default: 10000")
	copyKeyCount := flag.Int64("copyKeyCount", 1000, "We use redis pipeline to copy bunch of keys together, number of keys in pipeline to execute the pipeline, default: 1000")

	// used in deleteKeys
	delAfter := flag.Int64("delAfter", 10000, "We use redis pipeline to delete a bunch of keys together, number of keys in pipeline to initiate the pipeline delete, default: 10000")

	// client connection related flags
	delOldConn := flag.Bool("delOldConn", false, "Should this script delete old connections created by client, default: false")
	delOldConnAge := flag.Int64("delOldConnAge", 365, "The client connection older than n days will be killed, default: 365 (days)")
	delOldConnIdle := flag.Int64("delOldConnIdle", 180, "The client connection idle for n days will be killed, default: 180 (days)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	// This prints help and default values
	if *copyKeys == false && *deleteKeys == false && *printKeys == false && *populateData == false && *checkConnAge == false && *checkOldKey == false  {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	tm.Clear()
	fmt.Println("Executing...")

	s := connectSrcRedis(*srcRedisHost, *srcRedisDB)
	defer s.Close()

	pong, err := s.Ping().Result()
	check(err)
	fmt.Println("Checking Ping: ", pong)

	if *checkConnAge == true {
		connectionDetails := clientConnDetails(s)

		if *delOldConn == true {
			connToDel := parseClientConn(connectionDetails, *delOldConnAge, *delOldConnIdle)
			deleteClientConn(s, connToDel)
		} else {
			fmt.Println(connectionDetails)
		}
	}

	// channel for scan and getting key info
	ch := make(chan []string, 50)

	// after key info, if sixty day condition match, put keys in this channel
	och := make(chan string, 50)

	if *copyKeys == true || *deleteKeys == true || *printKeys == true || *checkOldKey == true {
		go scanKeys(s, ch, *scanCount, *keyMatch)
	}

	if *copyKeys == true {
		if *copyKeyCount < *scanCount {
			fmt.Println("scanCount can not be less than copyKeyCount")
			os.Exit(1)
		}
		start := time.Now()
		d := connectDstRedis(*dstRedisHost, *dstRedisDB)
		defer d.Close()
		go copyKey(s, d, ch, start, *copyKeyCount)
	}

	if *checkOldKey == true {
		go keyInfo(s, ch, och, *keyAge, *deleteKeys)
	}

	if *populateData == true {
		go populateRedis(s, *populateCount)
	}

	if *printKeys == true {
		go printScanKey(ch)
	}

	if *deleteKeys == true {
		go keyInfo(s, ch, och, *keyAge, *deleteKeys)
		go delKeys(s, och, *delAfter, *deleteKeys)
	}

	// :)
	var input string
	fmt.Scanln(&input)
}