package main

import (
	"flag"
	"fmt"
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

	"github.com/aarzilli/golua/lua"
)

type Timings []time.Duration

var (
	L     *lua.State
	LLock sync.Mutex

	dnsCache     = make(map[string]string, 0)
	dnsCacheLock sync.RWMutex

	timingChan = make(chan time.Duration, 64)

	errors = uint64(0)

	client *http.Client

	printing = false
)

func (t Timings) Len() int {
	return len(t)
}

func (t Timings) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t Timings) Less(i, j int) bool {
	return t[i] < t[j]
}

func luaPrint(L *lua.State) int {
	if printing {
		str := L.ToString(1)
		fmt.Println(str)
	}

	return 1
}

func resolveDomain(domain string) (string, error) {
	var host string
	var port string
	var ip string
	var ok bool
	var err error

	if strings.Contains(domain, ":") {
		host, port, err = net.SplitHostPort(domain)
		if err != nil {
			return "", err
		}
	} else {
		host = domain
	}

	if isIp := net.ParseIP(host); isIp != nil {
		return domain, nil
	}

	dnsCacheLock.RLock()
	{
		ip, ok = dnsCache[host]
	}
	dnsCacheLock.RUnlock()

	if !ok {
		ips, err := net.LookupIP(host)
		if err != nil {
			return "", err
		}

		if len(ips) == 0 {
			return "", fmt.Errorf("failed to lookup %s", host)
		}

		// If it's an ipv6 address we need brackets around it.
		if ipv4 := ips[0].To4(); ipv4 != nil {
			ip = ipv4.String()
		} else {
			ip = "[" + ips[0].String() + "]"
		}

		dnsCacheLock.Lock()
		{
			dnsCache[host] = ip
		}
		dnsCacheLock.Unlock()
	}

	if port != "" {
		return ip + ":" + port, nil
	} else {
		return ip, nil
	}
}

func cacheDial(network string, addr string) (net.Conn, error) {
	url, err := resolveDomain(addr)
	if err != nil {
		return nil, err
	}

	return net.Dial(network, url)
}

func do(req *http.Request) (res *http.Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	res, err = client.Do(req)

	return
}

func worker(n int, sleep time.Duration, stop chan struct{}) {
	state := "__state" + strconv.FormatInt(int64(n), 10)

	LLock.Lock()
	{
		L.CreateTable(0, 0)
		L.SetGlobal(state)
	}
	LLock.Unlock()

	for {
		var (
			next = time.Now().Add(sleep)
			req  *http.Request
		)

		LLock.Lock()
		{
			L.GetGlobal("request")
			if !L.IsFunction(-1) {
				log.Fatal(fmt.Errorf("request is not a function"))
			}

			L.GetGlobal(state)
			if err := L.Call(1, 1); err != nil {
				log.Fatal(err)
			}

			if !L.IsTable(-1) {
				log.Fatal(fmt.Errorf("request did not return a table but a %s", L.Typename(int(L.Type(-1)))))
			}

			L.PushString("method")
			L.GetTable(-2)

			if !L.IsString(-1) {
				log.Fatal(fmt.Errorf("method is not a string"))
			}

			method := L.ToString(-1)
			L.Pop(1)

			L.PushString("url")
			L.GetTable(-2)

			if !L.IsString(-1) {
				log.Fatal(fmt.Errorf("url is not a string"))
			}

			url := L.ToString(-1)
			L.Pop(1)

			var err error
			req, err = http.NewRequest(method, url, nil)
			if err != nil {
				log.Fatal(err)
			}

			L.PushString("headers")
			L.GetTable(-2)

			if !L.IsNoneOrNil(-1) {
				if !L.IsTable(-1) {
					log.Fatal(fmt.Errorf("headers is not a table"))
				}

				L.PushNil()

				for L.Next(-2) > 0 {
					name := L.ToString(-2)
					value := L.ToString(-1)
					L.Pop(1)

					req.Header.Add(name, value)
				}
			}

			L.Pop(2) // Pop the headers table and the return value off the stack.
		}
		LLock.Unlock()

		start := time.Now()

		if res, err := do(req); err != nil {
			log.Fatal(err)
		} else {
			timingChan <- time.Now().Sub(start)

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Fatal(err)
			}
			res.Body.Close()

			LLock.Lock()
			{
				L.GetGlobal("response")
				if !L.IsFunction(-1) {
					log.Fatal(fmt.Errorf("response is not a function"))
				}

				L.CreateTable(0, 2)

				L.PushString("status")
				L.PushInteger(int64(res.StatusCode))
				L.RawSet(-3)

				L.PushString("body")
				L.PushString(string(body))
				L.RawSet(-3)

				L.PushString("headers")
				L.CreateTable(0, len(res.Header))

				for name, values := range res.Header {
					L.PushString(strings.ToLower(name))
					L.CreateTable(len(values), 0)

					for i, value := range values {
						L.PushInteger(int64(i + 1))
						L.PushString(value)
						L.RawSet(-3)
					}

					L.RawSet(-3)
				}

				L.RawSet(-3)

				L.GetGlobal(state)
				if err := L.Call(2, 1); err != nil {
					log.Fatal(err)
				}

				if !L.IsBoolean(-1) || !L.ToBoolean(-1) {
					atomic.AddUint64(&errors, 1)
				}

				L.Pop(1) // Pop the return value of the stack.
			}
			LLock.Unlock()
		}

		if delay := next.Sub(time.Now()); delay <= 0 {
			select {
			case <-stop:
				return
			default:
			}
		} else {
			select {
			case <-stop:
				return
			case <-time.After(delay):
			}
		}
	}
}

func main() {
	cachedns := flag.Bool("cachedns", true, "Cache dns lookups (dns lookup time is included in the request time and might slow things down).")
	keepalive := flag.Bool("keepalive", true, "Use keepalive connections.")
	rps := flag.Int("rps", 10, "The maximum number of requests per second.")
	script := flag.String("script", "simple.lua", "The lua script to run.")
	workers := flag.Int("workers", 100, "Number of workers to use (number of concurrent requests).")
	flag.Parse()

	// Don't start more workers than we want to do requests per second.
	if *workers > *rps {
		*workers = *rps
	}

	sleep := time.Duration((float64(time.Second) * float64(*workers)) / float64(*rps))

	fmt.Printf("starting %d worker(s) for %d requests per second\n", *workers, *rps)
	fmt.Printf("press Ctrl+C to stop and print statistics\n")

	L = lua.NewState()
	L.OpenLibs()
	L.Register("print", luaPrint)

	if err := L.DoFile(*script); err != nil {
		log.Fatal(err)
	}

	dial := net.Dial
	if *cachedns {
		dial = cacheDial
	}

	client = &http.Client{
		Jar: nil,
		Transport: &http.Transport{
			DisableKeepAlives:   !(*keepalive),
			DisableCompression:  false,
			MaxIdleConnsPerHost: 64,
			Dial:                dial,
		},
	}

	stop := make(chan struct{})
	start := time.Now()

	for i := 0; i < *workers; i++ {
		go worker(i, sleep, stop)
	}

	timings := make(Timings, *workers)

	go func() {
		safe := true
		for {
			select {
			case timing := <-timingChan:
				if safe {
					timings = append(timings, timing)
				}
			case <-stop:
				safe = false
			}
		}
	}()

	r := 0
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

print:
	for {
		time.Sleep(time.Second)

		select {
		case <-c:
			break print
		default:
		}

		nr := len(timings)
		s := "      " + fmt.Sprint(time.Duration(time.Now().Sub(start)/time.Second)*time.Second)

		fmt.Printf("%s: %d requests\n", s[len(s)-6:], nr-r)
		r = nr
	}

	printing = false
	end := time.Now()

	// Stop all workers.
	close(stop)

	duration := time.Duration(end.Sub(start)/time.Second) * time.Second
	ps := float64(len(timings)) / (float64(duration) / float64(time.Second))

	sort.Sort(timings)

	fmt.Printf("\n%d requests in %v\n", len(timings), duration)
	fmt.Printf("%d error(s)\n", atomic.LoadUint64(&errors))
	fmt.Printf("requests/sec: %.2f\n", ps)
	fmt.Printf("latency distribution:\n")
	fmt.Printf("   50%% %v\n", timings[int(float64(len(timings))*0.50)])
	fmt.Printf("   75%% %v\n", timings[int(float64(len(timings))*0.75)])
	fmt.Printf("   90%% %v\n", timings[int(float64(len(timings))*0.90)])
	fmt.Printf("   99%% %v\n", timings[int(float64(len(timings))*0.99)])
	fmt.Printf("  100%% %v\n", timings[len(timings)-1])
}
