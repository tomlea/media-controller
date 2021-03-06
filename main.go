package main

import "github.com/cwninja/media-controller/tv"
import "github.com/cwninja/media-controller/server"
import "flag"
import "log"
import "fmt"
import "sync"
import "os"
import urlParser "net/url"
import "net/http"
import "net"
import "time"
import "strconv"

// A global wait group to decide when it's OK to exit the process.
var wg sync.WaitGroup

func main() {
	log.SetFlags(0)
	tvUrl := flag.String("tv", os.Getenv("TV_CONTROL_URL"), "URL for TV.")
	listenAddress := flag.String("listen", ":2222", "Address for server to listen on.")
	webListenAddress := flag.String("listen-web", ":8081", "Address for web server to listen on.")
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("Please provide at least a command")
	}

	if *tvUrl == "" {
		log.Fatal("Please provide a tv URL")
	}

	myTv := tv.NewTV(*tvUrl)
	command := flag.Arg(0)

	if command == "play" {
		file := flag.Arg(1)

		var url string
		if _, err := os.Stat(file); err == nil {
			url = makeFileRemotelyAccessibleToTv(myTv, file)
		} else {
			url = file
		}
		myTv.Play(url)

	} else if command == "pause" {
		myTv.Pause()
	} else if command == "stop" {
		myTv.Stop()
	} else if command == "status" {
		status := myTv.GetTransportInfo()
		posInfo := myTv.GetPositionInfo()
		fmt.Printf("Url: %s\n%s  -  Progress: %d/%d\n", posInfo.URI, status, posInfo.Position, posInfo.Duration)
	} else if command == "server" {
		s, err := server.New(&myTv, *listenAddress)
		if err != nil {
			log.Fatal(err)
		}
		go s.Start()

		httpServer := &http.Server{
			Addr:    *webListenAddress,
			Handler: server.GetRouter(&myTv),
		}
		go httpServer.ListenAndServe()

		wg.Add(1)
	} else if command == "seekby" {
		time, _ := strconv.Atoi(flag.Arg(1))
		myTv.SeekBy(time)
	} else if command == "seekto" {
		time, _ := strconv.Atoi(flag.Arg(1))
		myTv.SeekTo(time)
	} else {
		log.Fatal("Unknown command")
	}

	// We may have spun up a HTTP server, so wait for it to not be in use. If we
	// are serving a remote URL, we'll just return immediately.
	wg.Wait()
}

func findHostAndPort(remoteAddress string) string {
	// What host/port should we serve from? Because the TV needs to access
	// it, we make a brief connection out to the TV, and then use the
	// host/port that the OS connected out from. This way we don't need to do
	// any guess work about what can access to what!
	parsedRemoteAddress, err := urlParser.Parse(remoteAddress)
	if err != nil {
		log.Fatal(err)
	}

	connection, err := net.Dial("tcp", parsedRemoteAddress.Host)
	if err != nil {
		log.Fatal(err)
	}

	localAddress := connection.LocalAddr()
	connection.Close()

	return localAddress.String()
}

func serveFile(address string, file string) string {
	// As we are not playing a URL, we need to spin up a HTTP server to give
	// us somewhere to play from.
	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, file)
	})

	// Actually spin up the server
	go http.ListenAndServe(address, nil)

	return fmt.Sprintf("http://%s/file", address)
}

func waitForFilmToStop(myTv tv.TV) {
	for time.Sleep(5 * time.Second); myTv.GetTransportInfo() != tv.STATUS_STOPPED; {
		time.Sleep(time.Second)
	}
}

func makeFileRemotelyAccessibleToTv(myTv tv.TV, file string) string {
	url := serveFile(findHostAndPort(myTv.Url), file)
	wg.Add(1)
	go func() {
		waitForFilmToStop(myTv)
		wg.Done()
	}()
	return url
}
