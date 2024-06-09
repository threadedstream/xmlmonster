package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	uploadPath                = "/upload"
	readFilePath              = "/read"
	contentTypeTextXML        = "text/xml"
	contentTypeApplicationXML = "application/xml"

	certFileEnv = "CERT_FILE"
	keyFileEnv  = "KEY_FILE"
)

func mustObjectStorage(endpoint, accessKey, secretKey string) *minio.Client {
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal("failed to init object storage", err)
	}
	return cli
}

func main() {
	baseCtx := context.Background()
	ctx, globalCancel := signal.NotifyContext(baseCtx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	cli := mustObjectStorage("localhost:9000", "minioadmin", "minioadmin")

	serv := &http.Server{
		Addr:    ":8000",
		Handler: newXMLParseHandler(cli),
	}

	var certFile, keyFile string
	if certFile = os.Getenv(certFileEnv); certFile == "" {
		log.Fatal("Missing required environment variable: " + certFileEnv)
	}

	if keyFile = os.Getenv(keyFileEnv); keyFile == "" {
		log.Fatal("Missing required environment variable: " + keyFileEnv)
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
	if err := serv.ListenAndServeTLS(certFile, keyFile); err != nil {
		log.Fatal(err)
	}
}

type xmlParseHandler struct {
	cli      *minio.Client
	reqCount atomic.Int64
}

func newXMLParseHandler(cli *minio.Client) *xmlParseHandler {
	return &xmlParseHandler{cli: cli}
}

func (xph *xmlParseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case uploadPath:
		xph.handleUpload(w, r)
	case readFilePath:
		xph.handleRead(w, r)
	default:
		writeNotFound(w)
	}
}

func writeNotFound(w http.ResponseWriter) {
	_, _ = w.Write([]byte(
		"<p>Oops, you walked the wrong path</p>",
	))
}

func (xph *xmlParseHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if contentType := r.Header.Get("Content-Type"); contentType != contentTypeTextXML && contentType != contentTypeApplicationXML {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rawXML, err := io.ReadAll(r.Body)
	if err != nil {
		httpWrite(w, http.StatusInternalServerError, []byte("failed to read content from body"))
		return
	}

	bucketID := xph.composeBucketID()
	_, err = xph.cli.PutObject(r.Context(), "bucket1", bucketID, bytes.NewBuffer(rawXML), int64(len(rawXML)), minio.PutObjectOptions{})
	if err != nil {
		httpWrite(w, http.StatusInternalServerError, []byte("failed to upload object"))
		return
	}

	httpWrite(w, http.StatusOK, []byte(bucketID))
}

func (xph *xmlParseHandler) handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", contentTypeApplicationXML)

	bucketID := r.URL.Query().Get("bucket_id")
	if bucketID == "" {
		httpWrite(w, http.StatusBadRequest, []byte("bucket_id is required"))
		return
	}

	obj, err := xph.cli.GetObject(r.Context(), "bucket1", bucketID, minio.GetObjectOptions{})
	if err != nil {
		httpWrite(w, http.StatusInternalServerError, []byte("failed to get object"))
		return
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		httpWrite(w, http.StatusInternalServerError, []byte("failed to read object"))
	}

	httpWrite(w, http.StatusOK, data)
}

func (xph *xmlParseHandler) composeBucketID() string {
	return fmt.Sprintf(
		"xmlobject/%d",
		xph.reqCount.Add(1),
	)
}

func httpWrite(w http.ResponseWriter, statusCode int, data []byte) {
	w.WriteHeader(statusCode)
	if len(data) > 0 {
		_, _ = w.Write(data)
	}
}
