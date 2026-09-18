package health

import (
	"net/http"
	"time"
)

func NewServer(addr string, prober *Prober) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	ready := func(w http.ResponseWriter, _ *http.Request) {
		ok, err := prober.Ready()
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			if err != nil {
				_, _ = w.Write([]byte(err.Error() + "\n"))
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}

	mux.HandleFunc("GET /readyz", ready)
	mux.HandleFunc("GET /healthz", ready)

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
