package main 

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /",func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<h1>Chat Empleados</h1><p>Servidor Funcionando</p>"))
	})
	addr := ":6060"
	log.Printf("[INFO] Servidor Iniciando en http://localhost%s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[FATAL] Error Iniciando servidor: %v", err)
	}
}
