package main

import (
    "flag"
    "log"
    "net/http"
    "os"

    intrnl "termchat/internal"
)

func main() {
    addr := flag.String("addr", getEnv("TERMCHAT_ADDR", ":8080"), "server listen address")
    path := flag.String("path", getEnv("TERMCHAT_PATH", "/join"), "websocket join path")
    flag.Parse()

    hub := intrnl.NewHub()

    http.HandleFunc(*path, func(writer http.ResponseWriter, request *http.Request) {
        intrnl.ServeWS(hub, writer, request)
    })

    // simple HTTP endpoint to check for room existence without creating it
    http.HandleFunc("/exists", func(w http.ResponseWriter, r *http.Request) {
        room := r.URL.Query().Get("room")
        if room == "" {
            http.Error(w, "missing room", http.StatusBadRequest)
            return
        }
        if hub.Exists(room) {
            w.WriteHeader(http.StatusOK)
            _, _ = w.Write([]byte("ok"))
            return
        }
        http.Error(w, "not found", http.StatusNotFound)
    })

    log.Printf("TermChat server listening on %s%s", *addr, *path)
    if err := http.ListenAndServe(*addr, nil); err != nil {
        log.Fatalf("server error: %v", err)
    }
}

func getEnv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
