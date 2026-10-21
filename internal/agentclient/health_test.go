package agentclient

import (
 "context"
 "net"
 "net/http"
 "path/filepath"
 "testing"
)

func TestAgentHealth(t *testing.T){
 socket:=filepath.Join(t.TempDir(),"agent.sock")
 l,err:=net.Listen("unix",socket);if err!=nil{t.Fatal(err)}
 defer l.Close()
 s:=&http.Server{Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"status":"ok","component":"locas-agent"}`))})}
 defer s.Close();go func(){_=s.Serve(l)}()
 h,err:=Check(context.Background(),socket)
 if err!=nil||h.Component!="locas-agent"{t.Fatalf("health=%+v err=%v",h,err)}
}
func TestUnavailableAgent(t *testing.T){
 _,err:=Check(context.Background(),filepath.Join(t.TempDir(),"missing.sock"))
 if err==nil{t.Fatal("expected unavailable-agent error")}
}
func TestNoSocket(t *testing.T){if _,err:=Check(context.Background(),"");err==nil{t.Fatal("expected empty-path error")}}
