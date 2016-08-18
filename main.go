package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cjoudrey/gluahttp"
	"github.com/cjoudrey/gluaurl"
	"github.com/erikdubbelboer/hench/internal/ratelimit"
	"github.com/yuin/gopher-lua"

	gluajson "github.com/layeh/gopher-json"
)

var (
	L     *lua.LState
	LLock sync.Mutex

	durationChan = make(chan time.Duration, 0)

	errorsN = uint64(0)

	client *http.Client

	rate *ratelimit.Limiter

	start sync.WaitGroup
	stop  = make(chan lua.LValue, 0)
)

func luaPrint(L *lua.LState) int {
	fmt.Print(L.Get(1).String())

	return 0
}

func luaPrintln(L *lua.LState) int {
	fmt.Println(L.Get(1).String())

	return 0
}

func luaExit(L *lua.LState) int {
	c, _ := L.Get(1).(lua.LNumber)
	os.Exit(int(c))

	return 0
}

func luaStop(L *lua.LState) int {
	defer func() {
		recover()
	}()

	close(stop)

	return 0
}

func buildRequest(stateName string) *http.Request {
	LLock.Lock()
	defer LLock.Unlock()

	if err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal("request"),
		NRet:    1,
		Protect: true,
	}, L.GetGlobal(stateName)); err != nil {
		log.Fatal(err)
	}

	tableVal := L.Get(-1)

	if tableVal.Type() != lua.LTTable {
		return nil
	}

	table := tableVal.(*lua.LTable)

	method := table.RawGet(lua.LString("method"))
	url := table.RawGet(lua.LString("url"))
	body := table.RawGet(lua.LString("body"))

	var bodyReader io.Reader

	if body.Type() != lua.LTNil {
		bodyReader = strings.NewReader(body.String())
	}

	req, err := http.NewRequest(method.String(), url.String(), bodyReader)
	if err != nil {
		log.Fatal(err)
	}

	headers := table.RawGet(lua.LString("headers"))

	if h, ok := headers.(*lua.LTable); ok {
		h.ForEach(func(key, value lua.LValue) {
			req.Header.Add(key.String(), value.String())
		})
	}

	L.Pop(1)

	return req
}

func handleResponse(res *http.Response, stateName string) {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	res.Body.Close()

	LLock.Lock()
	defer LLock.Unlock()

	headers := L.NewTable()

	for name, values := range res.Header {
		header := L.NewTable()

		for _, value := range values {
			header.Append(lua.LString(value))
		}

		headers.RawSet(lua.LString(strings.ToLower(name)), header)
	}

	table := L.NewTable()

	table.RawSet(lua.LString("status"), lua.LNumber(res.StatusCode))
	table.RawSet(lua.LString("body"), lua.LString(body))
	table.RawSet(lua.LString("headers"), headers)

	if err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal("response"),
		NRet:    1,
		Protect: true,
	}, table, L.GetGlobal(stateName)); err != nil {
		log.Fatal(err)
	}

	if ok := L.Get(-1); ok.Type() != lua.LTBool || !bool(ok.(lua.LBool)) {
		atomic.AddUint64(&errorsN, 1)
	}

	L.Pop(1)
}

func worker(n int) {
	stateName := "__state" + strconv.FormatInt(int64(n), 10)

	LLock.Lock()
	{
		L.SetGlobal(stateName, L.CreateTable(0, 0))

		if workerCb := L.GetGlobal("worker"); workerCb.Type() == lua.LTFunction {
			if err := L.CallByParam(lua.P{
				Fn:      workerCb,
				NRet:    0,
				Protect: true,
			}, L.GetGlobal(stateName)); err != nil {
				log.Fatal(err)
			}
		}
	}
	LLock.Unlock()

	start.Wait()

	for {
		for {
			limit, sleep := rate.Try()
			if !limit {
				select {
				case <-stop:
					return
				default:
				}

				break
			}

			select {
			case <-stop:
				return
			case <-time.After(sleep):
			}
		}

		req := buildRequest(stateName)
		if req == nil {
			continue
		}

		startTime := time.Now()

		if res, err := client.Do(req); err != nil {
			atomic.AddUint64(&errorsN, 1)
		} else {
			durationChan <- time.Now().Sub(startTime)

			handleResponse(res, stateName)
		}
	}
}

func main() {
	cachedns := flag.Bool("cachedns", true,
		"Cache dns lookups (dns lookup time is included in the request time and might slow things down)")
	rps := flag.Int("rps", 10, "The maximum number of requests per second")
	script := flag.String("script", "", "Optional Lua script to run")
	workers := flag.Int("workers", 100,
		"Number of workers to use (number of concurrent requests)")
	keepalive := flag.Bool("keepalive", true, "Use keepalive connections")
	compression := flag.Bool("compression", true, "Enable or disable compression")
	flag.Parse()

	dial := net.Dial
	if *cachedns {
		dial = cacheDial
	}

	client = &http.Client{
		Jar: nil,
		Transport: &http.Transport{
			DisableKeepAlives:   !(*keepalive),
			DisableCompression:  !(*compression),
			MaxIdleConnsPerHost: *workers,
			Dial:                dial,
		},
	}

	L = lua.NewState()
	L.OpenLibs()
	L.Register("print", luaPrint)
	L.Register("println", luaPrintln)
	L.Register("exit", luaExit)
	L.Register("stop", luaStop)

	L.PreloadModule("http", gluahttp.NewHttpModule(client).Loader)
	L.PreloadModule("json", gluajson.Loader)
	L.PreloadModule("url", gluaurl.Loader)

	args := L.NewTable()
	for _, arg := range flag.Args() {
		args.Append(lua.LString(arg))
	}
	L.SetGlobal("args", args)

	if *script == "" {
		if err := L.DoString(`
			if #args == 0 then
				println('No url given')
				exit(1)
			end

			function request(state)
				return {
					['method' ] = 'GET',
					['url'    ] = args[1],
					['headers'] = {
						['User-Agent'] = 'hench',
					}
				}
			end

			function response(res, state)
				return res.status == 200
			end
		`); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := L.DoFile(*script); err != nil {
			log.Fatal(err)
		}
	}

	var workersWg sync.WaitGroup

	workersWg.Add(*workers)

	fmt.Printf("starting %d worker(s) for %d requests per second\n", *workers, *rps)
	fmt.Printf("press Ctrl+C to stop and print statistics\n")

	start.Add(1)
	for i := 0; i < *workers; i++ {
		go func(i int) {
			defer workersWg.Done()
			worker(i)
		}(i)
	}

	rate = ratelimit.New(float64(*rps), time.Second, 0)

	// Start ticking here so we won't have more than rps
	// requests after the first tick.
	secondTicker := time.Tick(time.Second)
	startTime := time.Now()

	start.Done()

	// Now all workers are firing request and we can start collecting durations.

	durations := make(Durations, 0)
	durationsN := uint64(0)

	go func() {
		for {
			select {
			case duration := <-durationChan:
				durations = append(durations, duration)
				atomic.AddUint64(&durationsN, 1)
			case <-stop:
				for range durationChan {
					// Do nothing, just consume so the workers don't hang.
				}
			}
		}
	}()

	// While collecting durations we should notify the user every second.

	lastDurationsN := uint64(0)
	lastErrorsN := uint64(0)

	// Wait for Ctrl+C to stop.
	c := make(chan os.Signal, 0)
	signal.Notify(c, os.Interrupt)

printFor:
	for {
		select {
		case <-c:
			break printFor
		case <-stop:
			break printFor
		case <-secondTicker:
			nowDurationsN := atomic.LoadUint64(&durationsN)
			nowErrorsN := atomic.LoadUint64(&errorsN)

			// Always format the time elapsed as exactly 6 characters.
			s := "      " + ((time.Now().Sub(startTime) / time.Second) * time.Second).String()
			fmt.Printf("%s: %d requests %d errors\n", s[len(s)-6:], nowDurationsN-lastDurationsN, nowErrorsN-lastErrorsN)

			lastDurationsN = nowDurationsN
			lastErrorsN = nowErrorsN
		}
	}

	stopTime := time.Now()

	// Stop all workers.
	// This might fail if one of the workers also closes the stop channel.
	// To prevent a panic we wrapt this close in a recover.
	func() {
		defer func() {
			recover()
		}()

		close(stop)
	}()

	// Wait until all workers are done.
	workersWg.Wait()

	duration := time.Duration(stopTime.Sub(startTime)/time.Millisecond) * time.Millisecond
	perSecond := float64(durationsN) / (float64(duration) / float64(time.Second))

	sort.Sort(durations)

	fmt.Printf("\n%d successful requests in %v\n", durationsN, duration)
	fmt.Printf("%d error(s)\n", atomic.LoadUint64(&errorsN))
	fmt.Printf("successful requests/sec: %.2f\n", perSecond)
	if durationsN > 0 {
		fmt.Printf("latency distribution:\n")
		fmt.Printf("   50%% %v\n", durations[int(float64(durationsN)*0.50)])
		fmt.Printf("   75%% %v\n", durations[int(float64(durationsN)*0.75)])
		fmt.Printf("   90%% %v\n", durations[int(float64(durationsN)*0.90)])
		fmt.Printf("   99%% %v\n", durations[int(float64(durationsN)*0.99)])
		fmt.Printf("  100%% %v\n", durations[durationsN-1])
	}
}
