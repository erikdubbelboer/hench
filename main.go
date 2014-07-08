
package main


import (
  "fmt"
  "log"
  "flag"
  "time"
  "sync"
  "sort"

  "os"
  "os/signal"

  "net/http"

  "github.com/aarzilli/golua/lua"
)


type Timings []time.Duration


var (
  verbose = flag.Bool("verbose", false, "Verbose output.")

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


func do(client *http.Client, req *http.Request) (res *http.Response, err error) {
  defer func() {
    if r := recover(); r != nil {
      err = fmt.Errorf("%v", r)
    }
  }()

  res, err = client.Do(req)

  return
}


func worker(sleep time.Duration, done chan struct{}) {
  L := lua.NewState()
  defer L.Close()
  L.OpenLibs()

  client := &http.Client{
    Jar      : nil,
    Timeout  : time.Second,
    Transport: &http.Transport{
      DisableKeepAlives: true,
      DisableCompression: false,
    },
  }

  for {
    next := time.Now().Add(sleep)


    if err := L.DoFile("random.lua"); err != nil {
      log.Fatal(err)
    }

    L.GetGlobal("request")
    if !L.IsTable(-1) {
      log.Fatal(fmt.Errorf("request is not a table"))
    }


    L.PushString("method")
    L.GetTable(-2)

    if !L.IsString(-1) {
      log.Fatal(fmt.Errorf("request.method is not a string"))
    }

    method := L.ToString(-1)
    L.Pop(1)


    L.PushString("url")
    L.GetTable(-2)

    if !L.IsString(-1) {
      log.Fatal(fmt.Errorf("request.url is not a string"))
    }

    url := L.ToString(-1)
    L.Pop(1)


    if *verbose {
      log.Printf("%s %s\n", method, url)
    }

    req, err := http.NewRequest(method, url, nil)
    if err != nil {
      log.Fatal(err)
    }


    L.PushString("headers")
    L.GetTable(-2)

    if !L.IsNoneOrNil(-1) {
      if !L.IsTable(-1) {
        log.Fatal(fmt.Errorf("request.headers is not a table"))
      }

      L.PushNil()

      for L.Next(-2) > 0 {
        name  := L.ToString(-2)
        value := L.ToString(-1)
        L.Pop(1)

        if *verbose {
          log.Printf("%s: %s\n", name, value)
        }

        req.Header.Add(name, value)
      }
    }


    L.Pop(1) // Pop the request table of the stack.


    start := time.Now()

    if res, err := do(client, req); err != nil {
      errors++
    } else if res.StatusCode != 200 {
      errors++
    } else {
      timing := time.Now().Sub(start)

      timingsLock.Lock()
      timings = append(timings, timing)
      timingsLock.Unlock()


      if *verbose {
        log.Println(res.Status)
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

          if *verbose {
            log.Printf("%s: %s\n", name, value)
          }
        }

        L.RawSet(-3)
      }

      L.RawSet(-3)

      L.SetGlobal("response")
    }

    delay := next.Sub(time.Now())

    if delay <= 0 {
      select {
      case <- done:
        return
      default:
      }
    } else {
      select {
      case <-done:
        return
      case <- time.After(next.Sub(time.Now())):
      }
    }
  }
}


func main() {
  rps := flag.Int("rps", 10, "Requests per second.")
  flag.Parse()

  procs := 100
  sleep := time.Duration((float64(time.Second) * float64(procs)) / float64(*rps))

  log.Printf("starting %d goroutines for %d requests per second\n", procs, *rps)

  done  := make([]chan struct{}, procs)
  start := time.Now()

  for i := 0; i < procs; i++ {
    done[i] = make(chan struct{}, 0)

    go worker(sleep, done[i])
  }


  c := make(chan os.Signal, 1)
  signal.Notify(c, os.Interrupt)
  for _ = range c {
    break
  }


  for _, d := range done {
    d <- struct{}{}
  }

  duration := time.Now().Sub(start)
  ps       := float64(len(timings)) / (float64(duration) / float64(time.Second))


  sort.Sort(timings)


  fmt.Printf("\n%d requests in %v\n%d errors\nrequests/sec: %.2f\nlatency distribution:\n", len(timings), duration, errors, ps)
  fmt.Printf("  50%% %v\n", timings[int(float64(len(timings)) * 0.50)])
  fmt.Printf("  75%% %v\n", timings[int(float64(len(timings)) * 0.75)])
  fmt.Printf("  90%% %v\n", timings[int(float64(len(timings)) * 0.90)])
  fmt.Printf("  99%% %v\n", timings[int(float64(len(timings)) * 0.99)])
}

