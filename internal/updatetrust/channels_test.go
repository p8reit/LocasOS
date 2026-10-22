package updatetrust

import "testing"

func TestSelectLatest(t *testing.T) {
 releases:=[]ReleaseCandidate{
  {Tag:"v0.9.0",Prerelease:false},
  {Tag:"v0.11.0",Prerelease:true},
  {Tag:"v0.10.0",Prerelease:false},
  {Tag:"v2.0.0",Draft:true},
  {Tag:"garbage",Prerelease:false},
 }
 stable,ok:=SelectLatest(releases,"stable")
 if !ok||stable.Tag!="v0.10.0"{t.Fatalf("stable: %#v %v",stable,ok)}
 early,ok:=SelectLatest(releases,"beta")
 if !ok||early.Tag!="v0.11.0"{t.Fatalf("early: %#v %v",early,ok)}
 // Promotion changes visibility only: same version becomes stable.
 releases[1].Prerelease=false
 stable,ok=SelectLatest(releases,"stable")
 if !ok||stable.Tag!="v0.11.0"{t.Fatalf("promoted: %#v %v",stable,ok)}
 early,ok=SelectLatest(releases,"beta")
 if !ok||early.Tag!="v0.11.0"{t.Fatalf("early after promotion: %#v %v",early,ok)}
}
func TestRejectUntrustedChannel(t *testing.T) {
 if _,ok:=SelectLatest([]ReleaseCandidate{{Tag:"v1.0.0"}},"dev");ok{t.Fatal("accepted unknown channel")}
 if _,err:=CompareVersions("v1.2.3","v1.2.4");err!=nil{t.Fatal(err)}
}
