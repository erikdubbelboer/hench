
package main


import (
  "fmt"
  "log"
  "flag"
  "time"
  "sync"
  "sort"
  "strconv"
  "runtime"
  "strings"

  "os"
  "os/signal"

  "net"
  "net/url"
  "net/http"

  "github.com/aarzilli/golua/lua"
)


type Timings []time.Duration


var (
  L     *lua.State
  LLock sync.Mutex

  dnsCache     = make(map[string]string, 0)
  dnsCacheLock sync.RWMutex

  timings     Timings
  timingsLock sync.Mutex

  errors = uint64(0)
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


func resolveUrl(surl string) (string, error) {
  var host string
  var port string
  var ip   string
  var ok   bool

  purl, err := url.Parse(surl)
  if err != nil {
    return "", err
  }

  if strings.Contains(purl.Host, ":") {
    host, port, err = net.SplitHostPort(purl.Host)
    if err != nil {
      return "", err
    }
  } else {
    host = purl.Host
  }

  if isIp := net.ParseIP(host); isIp != nil {
    return surl, nil
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

    ip = ips[0].String()

    dnsCacheLock.Lock()
    {
      dnsCache[host] = ip
    }
    dnsCacheLock.Unlock()
  }

  purl.Host = ip

  if port != "" {
    purl.Host += ":" + port
  }

  return purl.String(), nil
}


func do(client *http.Client, req *http.Request) (res *http.Response, err error) {
  defer func() {
    if r := recover(); r != nil {
      err = fmt.Errorf("%v", r)
    }
  }()

  res, err = client.Do(req)

  return
}


func worker(n int, sleep time.Duration, done chan struct{}) {
  state := "__state" + strconv.FormatInt(int64(n), 10)


  LLock.Lock()
  {
    L.CreateTable(0, 0)
    L.SetGlobal(state)
  }
  LLock.Unlock()

  client := &http.Client{
    Jar      : nil,
    Transport: &http.Transport{
      DisableKeepAlives    : true,
      DisableCompression   : false,
    },
  }

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
      L.Call(1, 1)

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

      url, err := resolveUrl(L.ToString(-1))
      if err != nil {
        log.Fatal(err)
      }
      L.Pop(1)


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
          name  := L.ToString(-2)
          value := L.ToString(-1)
          L.Pop(1)

          req.Header.Add(name, value)
        }
      }

      L.Pop(2) // Pop the headers table and the return value off the stack.
    }
    LLock.Unlock()


    start := time.Now()

    if res, err := do(client, req); err != nil {
      log.Println(err)
    } else {
      timing := time.Now().Sub(start)

      timingsLock.Lock()
      timings = append(timings, timing)
      timingsLock.Unlock()


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

        L.PushString("headers")
        L.CreateTable(0, len(res.Header))

        for name, values := range res.Header {
          L.PushString(name)
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
        L.Call(2, 1)

        if !L.IsBoolean(-1) || !L.ToBoolean(-1) {
          errors++
        }

        L.Pop(1) // Pop the return value of the stack.
      }
      LLock.Unlock()
    }

    if delay := next.Sub(time.Now()); delay <= 0 {
      select {
      case <- done:
        return
      default:
      }
    } else {
      select {
      case <-done:
        return
      case <- time.After(delay):
      }
    }
  }
}


func main() {
  workers := flag.Int   ("workers", runtime.GOMAXPROCS(0) * 20, "Number of workers to use (number of concurrent requests).")
  rps     := flag.Int   ("rps"    , 10                        , "Requests per second.")
  script  := flag.String("script" , "simple.lua"              , "The lua script to run.")
  flag.Parse()

  sleep := time.Duration((float64(time.Second) * float64(*workers)) / float64(*rps))

  fmt.Printf("starting %d workers for %d requests per second\n", *workers, *rps)

  L = lua.NewState()
  L.OpenLibs()

  if err := L.DoFile(*script); err != nil {
    log.Fatal(err)
  }

  done  := make([]chan struct{}, *workers)
  start := time.Now()

  for i := 0; i < *workers; i++ {
    done[i] = make(chan struct{}, 0)

    go worker(i, sleep, done[i])
  }


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

    timingsLock.Lock()
    nr := len(timings)
    timingsLock.Unlock()

    s := "      " + fmt.Sprint(time.Duration(time.Now().Sub(start) / time.Second) * time.Second)

    fmt.Printf("%s: %d requests\n", s[len(s) - 6:], nr - r)
    r = nr
  }


  // Stop all workers.
  for _, d := range done {
    d <- struct{}{}
  }


  duration := time.Duration(time.Now().Sub(start) / time.Second) * time.Second
  ps       := float64(len(timings)) / (float64(duration) / float64(time.Second))

  sort.Sort(timings)

  fmt.Printf("\n%d requests in %v\n"  , len(timings), duration)
  fmt.Printf("%d errors\n"            , errors)
  fmt.Printf("requests/sec: %.2f\n"   , ps)
  fmt.Printf("latency distribution:\n")
  fmt.Printf("   50%% %v\n", timings[int(float64(len(timings)) * 0.50)])
  fmt.Printf("   75%% %v\n", timings[int(float64(len(timings)) * 0.75)])
  fmt.Printf("   90%% %v\n", timings[int(float64(len(timings)) * 0.90)])
  fmt.Printf("   99%% %v\n", timings[int(float64(len(timings)) * 0.99)])
  fmt.Printf("  100%% %v\n", timings[len(timings) - 1])
}

