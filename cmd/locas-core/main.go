package main
import ("log"; "net/http"; "os"; "time"; "github.com/p8reit/LocasOS/internal/server")
func main() { addr:=os.Getenv("LOCAS_LISTEN"); if addr=="" {addr="127.0.0.1:8080"}; s:=&http.Server{Addr:addr,Handler:server.NewHandler("locas-core"),ReadHeaderTimeout:5*time.Second}; log.Printf("locas-core listening on %s",addr); log.Fatal(s.ListenAndServe()) }
