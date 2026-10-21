package appmanager

import (
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestRejectUnknownApp(t *testing.T) {
 m:=New(t.TempDir())
 r:=httptest.NewRequest(http.MethodPost,"/install",strings.NewReader(`{"app":"bash"}`))
 w:=httptest.NewRecorder()
 m.Handler().ServeHTTP(w,r)
 if w.Code!=400 {t.Fatalf("wanted 400 got %d",w.Code)}
}
func TestQueueAtMostOneJob(t *testing.T) {
 m:=New(t.TempDir())
 for _,want:=range []int{202,409} {
  r:=httptest.NewRequest(http.MethodPost,"/install",strings.NewReader(`{"app":"ollama"}`))
  w:=httptest.NewRecorder()
  m.Handler().ServeHTTP(w,r)
  if w.Code!=want {t.Fatalf("wanted %d got %d",want,w.Code)}
 }
}
