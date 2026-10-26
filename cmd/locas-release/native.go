package main

import (
 "crypto/ed25519"
 "encoding/base64"
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "regexp"
)

type nativeManifest struct {
 Version string `json:"version"`
 Commit string `json:"commit"`
 Platform string `json:"platform"`
 File string `json:"file"`
 SHA256 string `json:"sha256"`
}

func nativeSigning(version, commit, dir string) error {
 if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) {
  return fmt.Errorf("invalid version or commit")
 }
 key,err:=base64.StdEncoding.DecodeString(os.Getenv("LOCAS_RELEASE_ED25519_PRIVATE_KEY_B64"))
 if err!=nil || len(key)!=ed25519.PrivateKeySize {return fmt.Errorf("missing valid publisher signing key")}
 const archive="locasos-native-payload-amd64.tar"
 digest,err:=installHash(filepath.Join(dir,archive))
 if err!=nil{return err}
 body,err:=json.Marshal(nativeManifest{version,commit,"linux-amd64-native-vm",archive,digest})
 if err!=nil{return err}
 body=append(body,'\n')
 if err=os.WriteFile(filepath.Join(dir,"locasos-native.json"),body,0644);err!=nil{return err}
 return os.WriteFile(filepath.Join(dir,"locasos-native.sig"),ed25519.Sign(ed25519.PrivateKey(key),body),0644)
}
func nativeVerification(version,dir string) error {
 pub,err:=base64.StdEncoding.DecodeString(os.Getenv("LOCAS_UPDATE_PUBLIC_KEY_B64"))
 if err!=nil || len(pub)!=ed25519.PublicKeySize{return fmt.Errorf("trusted publisher public key required")}
 b,err:=os.ReadFile(filepath.Join(dir,"locasos-native.json"));if err!=nil{return err}
 if len(b)>4096{return fmt.Errorf("oversized manifest")}
 sig,err:=os.ReadFile(filepath.Join(dir,"locasos-native.sig"));if err!=nil{return err}
 if !ed25519.Verify(ed25519.PublicKey(pub),b,sig){return fmt.Errorf("invalid publisher signature")}
 var m nativeManifest
 if err=json.Unmarshal(b,&m);err!=nil{return err}
 if m.Version!=version || m.Platform!="linux-amd64-native-vm" || m.File!="locasos-native-payload-amd64.tar" ||
   !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(m.Commit) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(m.SHA256){
  return fmt.Errorf("invalid native payload identity")
 }
 digest,err:=installHash(filepath.Join(dir,m.File));if err!=nil{return err}
 if digest!=m.SHA256{return fmt.Errorf("native archive sha256 mismatch")}
 return nil
}
