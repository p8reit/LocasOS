package main

import (
	"archive/tar"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/p8reit/LocasOS/internal/updatetrust"
)

// Supplied only when building an official, signed release. Empty in developer builds.
var trustedPublicKeyBase64 string

const (
	requestFile = "/var/lib/locas/update-install.request"
	stateDir = "/var/lib/locas-updates"
	apiRoot = "https://api.github.com/repos/p8reit/LocasOS/releases/tags/"
	releaseRoot = "https://github.com/p8reit/LocasOS/releases/download/"
)

var tagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type request struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
}
type asset struct {
	Name string `json:"name"`
	URL string `json:"browser_download_url"`
	Size int64 `json:"size"`
}
type release struct {
	Tag string `json:"tag_name"`
	Draft bool `json:"draft"`
	Prerelease bool `json:"prerelease"`
	Assets []asset `json:"assets"`
}
type status struct {
	State string `json:"state"`
	Version string `json:"version,omitempty"`
	Channel string `json:"channel,omitempty"`
	Message string `json:"message,omitempty"`
}

func main() {
	if os.Geteuid()!=0 {fatal(errors.New("updater requires root"))}
	if err:=os.MkdirAll(stateDir,0755);err!=nil{fatal(err)}
	lock,err:=os.OpenFile(filepath.Join(stateDir,"update.lock"),os.O_CREATE|os.O_RDWR,0600)
	if err!=nil{fatal(err)}
	defer lock.Close()
	if err:=syscall.Flock(int(lock.Fd()),syscall.LOCK_EX|syscall.LOCK_NB);err!=nil{fatal(errors.New("update already running"))}
	defer syscall.Flock(int(lock.Fd()),syscall.LOCK_UN)
	buf,err:=os.ReadFile(requestFile)
	if errors.Is(err,os.ErrNotExist){return}
	if err!=nil{fatal(err)}
	if err:=os.Remove(requestFile);err!=nil{fatal(err)}
	if len(buf)>1024 {fatal(errors.New("invalid update request"))}
	var req request
	if err:=json.Unmarshal(buf,&req);err!=nil{fatal(err)}
	if !tagPattern.MatchString(req.Version) || (req.Channel!="stable"&&req.Channel!="beta"){
		fatal(errors.New("unsupported requested version or channel"))
	}
	fail:=func(err error){setStatus(status{State:"failed",Version:req.Version,Channel:req.Channel,Message:err.Error()});fatal(err)}
	key,err:=base64.StdEncoding.DecodeString(trustedPublicKeyBase64)
	if err!=nil || len(key)!=ed25519.PublicKeySize {fail(errors.New("trusted publisher key not configured; updates blocked"))}
	ctx,cancel:=context.WithTimeout(context.Background(),20*time.Minute)
	defer cancel()
	client:=&http.Client{Timeout:3*time.Minute,CheckRedirect:func(r *http.Request,via []*http.Request)error{
		if len(via)>6{return errors.New("too many release redirects")}
		h:=r.URL.Hostname()
		if r.URL.Scheme!="https" || (h!="github.com"&&!strings.HasSuffix(h,".githubusercontent.com")){
			return errors.New("untrusted release redirect")
		}
		return nil
	}}
	setStatus(status{State:"checking",Version:req.Version,Channel:req.Channel})
	metadata,err:=fetch(ctx,client,apiRoot+req.Version,1024*1024)
	if err!=nil{fail(err)}
	var rel release
	if err:=json.Unmarshal(metadata,&rel);err!=nil{fail(err)}
	if rel.Tag!=req.Version || rel.Draft || rel.Prerelease!=(req.Channel=="beta") {
		fail(errors.New("release channel or identity does not match approval"))
	}
	known:=map[string]asset{}
	for _,a:=range rel.Assets {
		switch a.Name{
		case "locasos-update.json","locasos-update.sig","locasos-upgrade-amd64.tar":
			expected:=releaseRoot+req.Version+"/"+a.Name
			if a.URL==expected && a.Size>0 && a.Size<=512*1024*1024 {known[a.Name]=a}
		}
	}
	for _,name:=range []string{"locasos-update.json","locasos-update.sig","locasos-upgrade-amd64.tar"} {
		if _,ok:=known[name];!ok{fail(fmt.Errorf("signed release missing %s",name))}
	}
	work,err:=os.MkdirTemp(stateDir,"stage-")
	if err!=nil{fail(err)}
	defer os.RemoveAll(work)
	setStatus(status{State:"downloading",Version:req.Version,Channel:req.Channel})
	for _,name:=range []string{"locasos-update.json","locasos-update.sig","locasos-upgrade-amd64.tar"} {
		a:=known[name]
		max:=a.Size+1
		if name=="locasos-update.json" && max>4096 {max=4096}
		if name=="locasos-update.sig" && max>ed25519.SignatureSize {max=ed25519.SignatureSize}
		if err:=download(ctx,client,a.URL,filepath.Join(work,name),max);err!=nil{fail(err)}
	}
	setStatus(status{State:"verifying",Version:req.Version,Channel:req.Channel})
	manifestBytes,err:=os.ReadFile(filepath.Join(work,"locasos-update.json"))
	if err!=nil{fail(err)}
	sig,err:=os.ReadFile(filepath.Join(work,"locasos-update.sig"))
	if err!=nil{fail(err)}
	manifest,err:=updatetrust.Verify(manifestBytes,sig,ed25519.PublicKey(key))
	if err!=nil{fail(err)}
	if manifest.Version!=req.Version {fail(errors.New("signed release version mismatch"))}
	archive:=filepath.Join(work,"locasos-upgrade-amd64.tar")
	digest,err:=updatetrust.FileHash(archive)
	if err!=nil{fail(err)}
	if !strings.EqualFold(digest,manifest.SHA256){fail(errors.New("signed archive digest mismatch"))}
	if err:=validateArchive(archive);err!=nil{fail(err)}
	sidecar:=filepath.Join(work,"locasos-upgrade-amd64.tar.sha256")
	if err:=os.WriteFile(sidecar,[]byte(digest+"  locasos-upgrade-amd64.tar\n"),0600);err!=nil{fail(err)}
	setStatus(status{State:"applying",Version:req.Version,Channel:req.Channel})
	command:=exec.CommandContext(ctx,"bash","/usr/local/sbin/locas-upgrade",archive)
	command.Stdout=os.Stdout
	command.Stderr=os.Stderr
	if err:=command.Run();err!=nil{fail(fmt.Errorf("upgrade failed; recovery attempted: %w",err))}
	versionFile:=filepath.Join(stateDir,"current-version")
	if err:=os.WriteFile(versionFile,[]byte(req.Version+"\n"),0644);err!=nil{fail(err)}
	setStatus(status{State:"installed",Version:req.Version,Channel:req.Channel,Message:"Health-checked upgrade completed"})
}

func fetch(ctx context.Context,c *http.Client,u string,max int64)([]byte,error){
	r,err:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil)
	if err!=nil{return nil,err}
	r.Header.Set("User-Agent","LOCAS-OS-signed-updater")
	resp,err:=c.Do(r)
	if err!=nil{return nil,err}
	defer resp.Body.Close()
	if resp.StatusCode!=http.StatusOK{return nil,fmt.Errorf("release download returned HTTP %d",resp.StatusCode)}
	out,err:=io.ReadAll(io.LimitReader(resp.Body,max+1))
	if err!=nil{return nil,err}
	if int64(len(out))>max{return nil,errors.New("release resource too large")}
	return out,nil
}
func download(ctx context.Context,c *http.Client,u,dest string,max int64) error {
	body,err:=fetch(ctx,c,u,max)
	if err!=nil{return err}
	return os.WriteFile(dest,body,0600)
}
func validateArchive(filename string)error{
	f,err:=os.Open(filename)
	if err!=nil{return err}
	defer f.Close()
	tr:=tar.NewReader(f)
	found:=false
	for {
		h,err:=tr.Next()
		if errors.Is(err,io.EOF){break}
		if err!=nil{return err}
		name:=strings.TrimPrefix(h.Name,"./")
		clean:=path.Clean(name)
		if name=="." || name=="" || strings.HasPrefix(name,"/") || clean!=name || strings.HasPrefix(name,"../"){
			return errors.New("unsafe path in update archive")
		}
		if !(name=="opt"||name=="etc"||name=="usr"||strings.HasPrefix(name,"opt/locas/")||
			strings.HasPrefix(name,"etc/systemd/system/")||strings.HasPrefix(name,"usr/local/sbin/")){
			return fmt.Errorf("unauthorized update path %s",name)
		}
		if h.Typeflag!=tar.TypeReg && h.Typeflag!=tar.TypeRegA && h.Typeflag!=tar.TypeDir {
			return errors.New("links and special files forbidden in update archive")
		}
		if name=="opt/locas/bin/locas-core" && h.Typeflag==tar.TypeReg{found=true}
	}
	if !found{return errors.New("missing LOCAS core executable")}
	return nil
}
func setStatus(s status){
	_ = os.MkdirAll(stateDir,0755)
	tmp,err:=os.CreateTemp(stateDir,"status-")
	if err!=nil{return}
	data,_:=json.Marshal(s)
	_,_=tmp.Write(append(data,'\n'))
	_=tmp.Chmod(0644)
	_=tmp.Close()
	_=os.Rename(tmp.Name(),filepath.Join(stateDir,"status.json"))
}
func fatal(err error){fmt.Fprintln(os.Stderr,err);os.Exit(1)}
