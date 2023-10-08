# WPSEV - обертка для работы с net/http
+ Имеет удобный роутер, с указанием http методов.
+ Есть возможность задавать динамические url
+ Мидалвары можно указывать списком, а не заворачивать один в другой
+ Поддержка мультипаттерна
+ Есть возможность выбора HTTP протокола
+ Сервер настраивается стандартными средствами net/http

### Пример простого сервера свыше перечисленными возможностями.

```go

func main() {

    middleware := func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("this middleware\n"))
    }

	myServer := wpsev.NewServer(&wpsev.Server{
		PortHTTP:        8888,
		PortHTTPS:       8443,
		PortHTTP3:       8445,
		ListenAddress:   "0.0.0.0",
		CertPath:        "wb.ru.crt",
		KeyPath:         "wb.ru.key",
		ShutdownTimeout: 2,
		EnableMTLS:      true //on mtls
	})



    myServer.AddRouter(http.MethodGet, "/",myServer.EnableMTLS, middleware, Home)
    myServer.AddRouter(http.MethodPost, "/", middleware, Home)
    myServer.AddRouter(http.MethodGet, "/person/:name/:age", middleware, Person)
    myServer.AddRouter(http.MethodGet, "/*file", middleware, File)
    myServer.AddRouter(http.MethodGet, "/upload/:id/*file", middleware, Upload)
	
	go 	myServer.Start()
	
    osSignalsCh := make(chan os.Signal, 1)
    signal.Notify(osSignalsCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
    <-osSignalsCh

    err := myServer.Stop()
    if err != nil {
        panic(err)
    }

}

func Upload(w http.ResponseWriter, r *http.Request) {
    pattern := wpsev.GetParam(r, "pattern")
    file := wpsev.GetParam(r, "file")
    id := wpsev.GetParam(r, "id")
    w.Write([]byte(fmt.Sprintf("pattern:%s\nid:%s\nfile:%s", pattern, id, file)))
}

func Person(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Pattern: " + wpsev.GetParam(r, "pattern") + "\n"))
    w.Write([]byte("Name: " + wpsev.GetParam(r, "name") + "\n"))
    w.Write([]byte("Age: " + wpsev.GetParam(r, "age") + "\n"))
}

func File(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Pattern: " + wpsev.GetParam(r, "pattern") + "\n"))
    w.Write([]byte("File: " + wpsev.GetParam(r, "file")))
}

func Home(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Home " + r.Method))
}
```
