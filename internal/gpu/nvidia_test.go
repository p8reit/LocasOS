package gpu
import "testing"
func TestParseCSV(t *testing.T){d,e:=ParseCSV("NVIDIA GeForce RTX 3060, 12288, 4096, 25\n");if e!=nil||len(d)!=1||d[0].MemoryUsedMiB!=4096{t.Fatalf("%v %v",d,e)}}
func TestReject(t *testing.T){for _,s:=range []string{"GPU, 100, 200, 5","GPU, 100, 50, 120","GPU, ??, 5, 2"}{if _,e:=ParseCSV(s);e==nil{t.Errorf("accepted %q",s)}}}
