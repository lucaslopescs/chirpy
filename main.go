package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(
		`<html>
  			<body>
    			<h1>Welcome, Chirpy Admin</h1>
    			<p>Chirpy has been visited %d times!</p>
  			</body>
		</html>`,
		cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) handleReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cfg.fileserverHits.Store(0)
}

func handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

type chirp struct {
	Body string `json:"body"`
}

func handleChirpsValidate(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	chirps := chirp{}
	err := decoder.Decode(&chirps)

	if err != nil {
		log.Printf("Error decoding chirps: %s", err)
		respondWithError(w, 400, err.Error())
		return
	}
	if len(chirps.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}
	type resp struct {
		Cleaned_body string `json:"cleaned_body"`
	}
	res := resp{
		Cleaned_body: profanFilter(chirps.Body),
	}
	respondWithJSON(w, 200, res)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	responseError := errorResponse{
		Error: msg,
	}
	respondWithJSON(w, code, responseError)

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func profanFilter(chirps string) string {
	naughtyWords := []string{"kerfuffle", "sharbert", "fornax"}
	var cleanChirp []string

	chirpSlice := strings.Fields(chirps)
	for i := 0; i < len(chirpSlice); i++ {
		if slices.Contains(naughtyWords, strings.ToLower(chirpSlice[i])) {
			cleanChirp = append(cleanChirp, "****")
			continue
		}
		cleanChirp = append(cleanChirp, chirpSlice[i])

	}
	cleanChirpString := strings.Join(cleanChirp, " ")
	return cleanChirpString

}

func main() {
	const filepathRoot = "."
	const port = "8080"

	var apiCfg apiConfig
	mux := http.NewServeMux()
	mux.Handle("/app/",
		apiCfg.middlewareMetricsInc(
			http.StripPrefix(
				"/app",
				http.FileServer(
					http.Dir(
						filepathRoot)))))

	mux.HandleFunc("GET /api/healthz", handler)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handleMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handleReset)
	mux.HandleFunc("POST /api/validate_chirp", handleChirpsValidate)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
}
