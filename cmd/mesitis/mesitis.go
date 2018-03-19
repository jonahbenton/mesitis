package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/jonahbenton/mesitis/pkg"
	"github.com/jonahbenton/mesitis/pkg/controller"

	"github.com/golang/glog"
)

func init() {
	//	flag.StringVar(&options.TLSCert, "tlsCert", "", "base-64 encoded PEM block to use as the certificate for TLS. If '--tlsCert' is used, then '--tlsKey' must also be used. If '--tlsCert' is not used, then TLS will not be used.")
	//	flag.StringVar(&options.TLSKey, "tlsKey", "", "base-64 encoded PEM block to use as the private key matching the TLS certificate. If '--tlsKey' is used, then '--tlsCert' must also be used")
	flag.Parse()
}

func main() {
	// TODO differentiate between command line run, print to stdout, and glog
	if flag.Arg(0) == "version" {
		fmt.Printf("%s/%s\n", path.Base(os.Args[0]), pkg.VERSION)
		return
	}
	//	if (options.TLSCert != "" || options.TLSKey != "") &&
	//		(options.TLSCert == "" || options.TLSKey == "") {
	//		fmt.Println("To use TLS, both --tlsCert and --tlsKey must be used")
	//		return
	//	}

	// TODO make a config object for redis
	storageType := getEnv("STORAGE_TYPE", "memory")
	var storage controller.Storage

	switch storageType {
	case "memory":
		storage = controller.NewMemStorage()
	case "redis":
		db := getEnv("STORAGE_REDIS_DATABASE", "0")
		database, err := strconv.Atoi(db)
		if err != nil {
			glog.Fatalf("Invalid STORAGE_REDIS_DATABASE: %s", err)
		}
		address := getEnv("STORAGE_REDIS_ADDRESS", "UNKNOWN")
		password := getEnv("STORAGE_REDIS_PASSWORD", "")
		storage = controller.NewRedisStorage(address, password, database)

	default:
		glog.Fatalf("Invalid STORAGE_TYPE: %s", storageType)
	}

	name := getEnv("POD_NAME", "UNKNOWN")
	namespace := getEnv("POD_NAMESPACE", "UNKNOWN")
	tmpdir := getEnv("TMPDIR", "/unknown")

	c := controller.CreateProductionController(name, namespace, storage, tmpdir)

	w := controller.CreateHTTPWrapper(c)

	listenOn := getEnv("LISTEN_ON", ":8080")
	g := getEnv("GRACEFUL_SECS", "10")
	gracefulSeconds, err := strconv.Atoi(g)
	if err != nil {
		glog.Fatalf("Invalid GRACEFUL_SECS: %s", err)
	}

	glog.Infof("Starting server %s on %s in namespace %s using storageType %s\n", name, listenOn, namespace, storageType)
	server := &http.Server{
		Addr: listenOn,
		//        WriteTimeout: time.Second * 15,
		//        ReadTimeout:  time.Second * 15,
		//        IdleTimeout:  time.Second * 60,
		Handler: w,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelOnInterrupt(ctx, cancelFunc)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), time.Duration(gracefulSeconds)*time.Second)
		defer cancel()
		if server.Shutdown(c) != nil {
			server.Close()
		}
	}()
	err = server.ListenAndServe()

	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		glog.Fatalln(err)
	}
	os.Exit(0)
}

// cancelOnInterrupt calls f when os.Interrupt or SIGTERM is received.
// It ignores subsequent interrupts on purpose - program should exit correctly after the first signal.
func cancelOnInterrupt(ctx context.Context, f context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
		case <-c:
			f()
		}
	}()
}

func getEnv(name string, def string) string {
	var v string
	if v = os.Getenv(name); v == "" {
		return def
	}
	return v
}
