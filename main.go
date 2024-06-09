package main

import (
	"context"
	"encoding/xml"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

const (
	uploadPath                = "/upload"
	contentTypeTextXML        = "text/xml"
	contentTypeApplicationXML = "application/xml"
)

type xmlPayload struct {
	UserFrom string `xml:"UserFrom"`
	UserTo   string `xml:"UserTo"`
	Message  string `xml:"Message"`
}

func main() {
	baseCtx := context.Background()
	ctx, globalCancel := signal.NotifyContext(baseCtx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	serv := &http.Server{
		Addr:    ":8000",
		Handler: &xmlParseHandler{},
	}

	go func() {
		select {
		case <-ctx.Done():
			globalCancel()

			timeoutCtx, cancel := context.WithTimeout(baseCtx, time.Second*5)
			defer cancel()
			if err := serv.Shutdown(timeoutCtx); err != nil {
				log.Println("http server shutdown err", err)
			}
		}
	}()

	log.Println("starting server with addr", serv.Addr)
	// spinning up http server
	if err := serv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

type xmlParseHandler struct{}

func (xph *xmlParseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != uploadPath {
		writeNotFound(w)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if contentType := r.Header.Get("Content-Type"); contentType != contentTypeTextXML && contentType != contentTypeApplicationXML {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	handleUpload(w, r)
}

func writeNotFound(w http.ResponseWriter) {
	_, _ = w.Write([]byte(
		"<p>Oops, you walked the wrong path</p>",
	))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	decoder := xml.NewDecoder(r.Body)
	v := xmlPayload{}
	if err := decoder.Decode(&v); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
