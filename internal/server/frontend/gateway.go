package gateway

import (
	"embed"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	matchserver "github.com/Ekotlikoff/gochess/internal/server/backend/match"
	"github.com/gofrs/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// SessionCache is the cache of all user sessions
	SessionCache *TTLMap
	// Our static web server content.
	//go:embed static
	webStaticFS embed.FS
)

func init() {
	SessionCache = NewTTLMap(50, 1800, 10)
}

// Credentials are the credentialss for authentication
type Credentials struct {
	Username string
}

// Serve static files and proxy to the different backends
func Serve(
	httpBackend *url.URL, websocketBackend *url.URL, port int, logFile *string,
	quiet bool,
) {
	if logFile != nil {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(file)
	}
	if quiet {
		log.SetOutput(ioutil.Discard)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleWebRoot)
	mux.HandleFunc("/session", StartSession)
	// HTTP backend proxying
	mux.Handle("/http/match", httputil.NewSingleHostReverseProxy(httpBackend))
	mux.Handle("/http/sync", httputil.NewSingleHostReverseProxy(httpBackend))
	mux.Handle("/http/async", httputil.NewSingleHostReverseProxy(httpBackend))
	// Websocket backend proxying
	mux.Handle("/ws", httputil.NewSingleHostReverseProxy(websocketBackend))
	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())
	log.Println("Gateway server listening on port", port, "...")
	http.ListenAndServe(":"+strconv.Itoa(port), mux)
}

// SetQuiet logging
func SetQuiet() {
	log.SetOutput(ioutil.Discard)
}

func handleWebRoot(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/static" + r.URL.Path // This is a hack to get the embedded path
	http.FileServer(http.FS(webStaticFS)).ServeHTTP(w, r)
}

// StartSession credit to https://www.sohamkamani.com/blog/2018/03/25/golang-session-authentication/
func StartSession(w http.ResponseWriter, r *http.Request) {
	log.SetPrefix("StartSession: ")
	var creds Credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		log.Println("Bad request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	} else if creds.Username == "" {
		log.Println("Missing username")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing username"))
		return
	}
	// Create a new random session token
	sessionToken, err := uuid.NewV4()
	if err != nil {
		log.Println("Failed to generate session token")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	sessionTokenStr := sessionToken.String()
	player := matchserver.NewPlayer(creds.Username)
	SessionCache.Put(sessionTokenStr, player)
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   sessionTokenStr,
		Expires: time.Now().Add(1800 * time.Second),
	})
}
