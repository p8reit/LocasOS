package docker
import ("context";"net";"net/http";"os";"path/filepath";"testing")
func TestInventory(t *testing.T){
 socket:=filepath.Join(t.TempDir(),"docker.sock");l,e:=net.Listen("unix",socket);if e!=nil{t.Fatal(e)};defer l.Close()
 server:=&http.Server{Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.Method!="GET"||r.URL.Path!="/v1.41/containers/json"{t.Errorf("unexpected request %s %s",r.Method,r.URL.Path)};w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`[{"Id":"abc","Names":["/ollama"],"Image":"ollama","State":"running"}]`))})}
 defer server.Close();go func(){_ =server.Serve(l)}()
 items,e:=List(context.Background(),socket);if e!=nil||len(items)!=1||items[0].ID!="abc"{t.Fatalf("items=%v err=%v",items,e)}
 _=os.Remove(socket)
}
func TestMissingSocket(t *testing.T){if _,e:=List(context.Background(),"");e==nil{t.Fatal("empty socket should fail")}}
