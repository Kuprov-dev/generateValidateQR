package main

import (
	"context"
	"fmt"
	"generateValidateQR/pkg/conf"
	"generateValidateQR/pkg/db"
	"generateValidateQR/pkg/generate"
	logging "generateValidateQR/pkg/logging"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

const PORT string = ":8081"
func main(){
	config := conf.New()


	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	logEntry := logrus.NewEntry(log)

	userDAO := db.InMemroyUserDAO{}

	r := mux.NewRouter()
	r.Handle("/generate",generate.GenerateHandler(config, &userDAO, context.TODO())).Methods("POST")
	r.Handle("/alidate",generate.ValidateTokensInBodyHandler(config, &userDAO))
	fmt.Println("Server started...")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	handler := logging.LoggingMiddleware(logEntry)(r)
	s := &http.Server{
		Addr:    PORT,
		Handler: handler,

		// So because the WriteTimeout was set pprof yielded an error,
		// that is why, and due to redundancy, setting timeouts was commented
		// IdleTimeout:  10 * time.Second,
		// ReadTimeout:  time.Second,
		// WriteTimeout: time.Second,
	}
	defer s.Close()

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println(err)

		}
	}()

	<-stop

	fmt.Println("Server stopped...")
}