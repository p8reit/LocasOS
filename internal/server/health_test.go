package server
import ("net/http";"net/http/httptest";"strings";"testing")
func TestHealth(t *testing.T) {w:=httptest.NewRecorder();NewHandler("locas-core").ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/healthz",nil));if w.Code!=http.StatusOK||!strings.Contains(w.Body.String(),`"component":"locas-core"`){t.Fatalf("unexpected health response: %d %s",w.Code,w.Body.String())}}
func TestNoManagementEndpoint(t *testing.T) {w:=httptest.NewRecorder();NewHandler("locas-core").ServeHTTP(w,httptest.NewRequest(http.MethodPost,"/api/v1/applications",nil));if w.Code!=http.StatusNotFound{t.Fatalf("got %d, want 404",w.Code)}}
