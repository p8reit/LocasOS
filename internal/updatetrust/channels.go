package updatetrust

import (
 "fmt"
 "regexp"
 "strconv"
)

type ReleaseCandidate struct {
 Tag string
 Draft bool
 Prerelease bool
}
var numericVersion = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

func VersionParts(tag string) ([3]int, bool) {
 var version [3]int
 m:=numericVersion.FindStringSubmatch(tag)
 if m==nil{return version,false}
 for i:=0;i<3;i++ {
  n,err:=strconv.Atoi(m[i+1]);if err!=nil{return version,false}
  version[i]=n
 }
 return version,true
}
func CompareVersions(a,b string) (int,error) {
 x,ok:=VersionParts(a);if !ok{return 0,fmt.Errorf("invalid version %q",a)}
 y,ok:=VersionParts(b);if !ok{return 0,fmt.Errorf("invalid version %q",b)}
 for i:=0;i<3;i++ {if x[i]<y[i]{return -1,nil};if x[i]>y[i]{return 1,nil}}
 return 0,nil
}
// SelectLatest chooses the highest version allowed by the channel. The same
// release may be visible in early access and later stable without rebuilding.
func SelectLatest(releases []ReleaseCandidate,channel string) (ReleaseCandidate,bool) {
 var chosen ReleaseCandidate
 found:=false
 if channel!="stable" && channel!="beta"{return chosen,false}
 for _,release:=range releases {
  if release.Draft || (channel=="stable" && release.Prerelease){continue}
  if _,ok:=VersionParts(release.Tag);!ok{continue}
  if !found{chosen=release;found=true;continue}
  cmp,err:=CompareVersions(release.Tag,chosen.Tag)
  if err==nil&&cmp>0{chosen=release}
 }
 return chosen,found
}
