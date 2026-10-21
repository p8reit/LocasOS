package docker
import ("context";"encoding/json";"errors";"net";"net/http";"time")
type Container struct {ID string `json:"Id"`; Names []string `json:"Names"`; Image string `json:"Image"`; State string `json:"State"`}
func List(ctx context.Context,socket string)([]Container,error){
 if socket==""{return nil,errors.New("Docker socket path required")}
 tr:=&http.Transport{DialContext:func(c context.Context,_,_ string)(net.Conn,error){return (&net.Dialer{Timeout:3*time.Second}).DialContext(c,"unix",socket)}}
 defer tr.CloseIdleConnections();client:=&http.Client{Transport:tr,Timeout:5*time.Second}
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://docker/v1.41/containers/json?all=1",nil);if err!=nil{return nil,err}
 resp,err:=client.Do(req);if err!=nil{return nil,err};defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK{return nil,errors.New("Docker inventory request failed")}
 var containers []Container;if err:=json.NewDecoder(http.MaxBytesReader(nil,resp.Body,1<<20)).Decode(&containers);err!=nil{return nil,err};return containers,nil
}
