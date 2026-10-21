package hardware
import ("bufio";"errors";"os";"runtime";"strconv";"strings")
type Snapshot struct {Node string `json:"node"`; CPUs int `json:"cpus"`; MemoryTotalMiB uint64 `json:"memory_total_mib"`; MemoryAvailableMiB uint64 `json:"memory_available_mib"`; GPUs []any `json:"gpus"`}
func ParseMeminfo(s string)(uint64,uint64,error){
 var total,available uint64;seen:=map[string]bool{};scanner:=bufio.NewScanner(strings.NewReader(s))
 for scanner.Scan(){f:=strings.Fields(scanner.Text());if len(f)<2{continue};key:=strings.TrimSuffix(f[0],":");if key!="MemTotal"&&key!="MemAvailable"{continue};if seen[key]{return 0,0,errors.New("duplicate memory field")};seen[key]=true;n,e:=strconv.ParseUint(f[1],10,64);if e!=nil{return 0,0,e};if len(f)>2&&f[2]!="kB"{return 0,0,errors.New("unexpected memory unit")};if key=="MemTotal"{total=n}else{available=n}}
 if err:=scanner.Err();err!=nil{return 0,0,err};if !seen["MemTotal"]||!seen["MemAvailable"]||available>total{return 0,0,errors.New("invalid meminfo")};return total/1024,available/1024,nil
}
func Collect()(Snapshot,error){host,err:=os.Hostname();if err!=nil{return Snapshot{},err};data,err:=os.ReadFile("/proc/meminfo");if err!=nil{return Snapshot{},err};total,available,err:=ParseMeminfo(string(data));if err!=nil{return Snapshot{},err};return Snapshot{Node:host,CPUs:runtime.NumCPU(),MemoryTotalMiB:total,MemoryAvailableMiB:available,GPUs:[]any{}},nil}
