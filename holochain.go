package holochain

import (
	_ "fmt"
	"os"
	"io/ioutil"
	"errors"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"path/filepath"
	"github.com/google/uuid"
	"github.com/BurntSushi/toml"
)

const Version string = "0.0.1"

const (
	DirectoryName string = ".holochain"
	DNAFileName string = "dna.conf"
	LocalFileName string = "local.conf"
	SysFileName string = "system.conf"
	PubKeyFileName string = "pub.key"
	PrivKeyFileName string = "priv.key"
	ChainFileName string = "chain.db"


)

type Config struct {
	Port string
	PeerModeAuthor bool
	PeerModeDHTNode bool
}

type Holochain struct {
	Id uuid.UUID
	ShortName string
	FullName string
	Description string
	Types []string
	Schemas map[string]string
	Validators map[string]string
}

//IsInitialized checks a path for a correctly set up .holochain directory
func IsInitialized(path string) bool {
	root := path+"/"+DirectoryName
	return dirExists(root) && fileExists(root+"/"+SysFileName)
}

//IsConfigured checks a directory for correctly set up holochain configuration files
func IsConfigured(path string) error {
	p := path+"/"+DNAFileName
	if !fileExists(p) {return errors.New("missing "+p)}
	lh, err := Load(path)
	if err != nil {return err}
	SelfDescribingSchemas := map[string]bool {
		"JSON": true}
	for _,t := range lh.Types {
		s := lh.Schemas[t]
		if !SelfDescribingSchemas[s] {
			if !fileExists(path+"/"+s) {return errors.New("DNA specified schema missing: "+s)}
		}
		s = lh.Validators[t]
		if !fileExists(path+"/"+s) {return errors.New("DNA specified validator missing: "+s)}
	}
	return nil
}

// New creates a new holochain structure with a randomly generated ID and default values
func New() Holochain {
	u,err := uuid.NewUUID()
	if err != nil {panic(err)}
	h := Holochain {
		Id:u,
		Types: []string{"myData"},
		Schemas: map[string]string{"myData":"JSON"},
		Validators: map[string]string{"myData":"valid_myData.js"},
	}
	return h
}

// Load creates a holochain structure from the configuration files
func Load(path string) (h Holochain,err error) {
	_,err = toml.DecodeFile(path+"/"+DNAFileName, &h)
	return h,err
}

// MarshalPublicKey stores a PublicKey to a serialized x509 format file
func MarshalPublicKey(path string, file string, key *ecdsa.PublicKey) error {
	k,err := x509.MarshalPKIXPublicKey(key)
	if err != nil {return err}
	err = writeFile(path,file,k)
	return err
}

// UnmarshalPublicKey loads a PublicKey from the serialized x509 format file
func UnmarshalPublicKey(path string, file string) (key *ecdsa.PublicKey,err error) {
	k,err := readFile(path,file)
	if (err != nil) {return nil,err}
	kk,err := x509.ParsePKIXPublicKey(k)
	key = kk.(*ecdsa.PublicKey)
	return key,err
}

// MarshalPrivateKey stores a PublicKey to a serialized x509 format file
func MarshalPrivateKey(path string, file string, key *ecdsa.PrivateKey) error {
	k,err := x509.MarshalECPrivateKey(key)
	if err != nil {return err}
	err = writeFile(path,file,k)
	return err
}

// UnmarshalPrivateKey loads a PublicKey from the serialized x509 format file
func UnmarshalPrivateKey(path string, file string) (key *ecdsa.PrivateKey,err error) {
	k,err := readFile(path,file)
	if (err != nil) {return nil,err}
	key,err = x509.ParseECPrivateKey(k)
	return key,err
}

// GenKeys creates a new pub/priv key pair and stores them at the given path.
func GenKeys(path string) error {
	if fileExists(path+"/"+PrivKeyFileName) {return errors.New("keys already exist")}
	priv,err := ecdsa.GenerateKey(elliptic.P256(),rand.Reader)
	if err != nil {return err}

	err = MarshalPrivateKey(path,PrivKeyFileName,priv)
	if err != nil {return err}

	var pub *ecdsa.PublicKey
	pub = priv.Public().(*ecdsa.PublicKey)
	err = MarshalPublicKey(path,PubKeyFileName,pub)
	return err
}

// GenChain sets up a holochain by creating the initial genesis links.
// It assumes a properly set up .holochain sub-directory with a config file and
// keys for signing.  See Gen
func GenChain() (err error) {
	return errors.New("not implemented")
}

//Init initializes service defaults and a new key pair in the dirname directory
func Init(path string) error {
	p := path+"/"+DirectoryName
	if err := os.MkdirAll(p,os.ModePerm); err != nil {
		return err
	}
	c := Config {
		PeerModeAuthor:true,
	}
	err := writeToml(p,SysFileName,c)
	if err != nil {return err}

	err = GenKeys(p)

	return err
}

// Gen adds template files suitable for editing to the given path
func GenDev(path string) (hP *Holochain, err error) {
	var h Holochain
	if err := os.MkdirAll(path,os.ModePerm); err != nil {
		return nil,err;
	}

	h = New()
	h.ShortName = filepath.Base(path)
	if err = writeToml(path,DNAFileName,h); err != nil {return}
	hP = &h
	//	if err = writeFile(path,"myData.cp",[]byte(s)); err != nil {return}  //if captain proto...
	s := `
function validateEntry(entry) {
    return true;
};
function validateChain(entry,user_data) {
    return true;
};
`
	if err = writeFile(path,"valid_myData.js",[]byte(s)); err != nil {return}
	return
}

/*func Link(h *Holochain, data interface{}) error {

}*/

// ConfiguredChains returns a list of the configured chains in the given holochain directory
func ConfiguredChains(root string) map[string]bool {
	files, _ := ioutil.ReadDir(root)
	chains := make(map[string]bool)
	for _, f := range files {
		if f.IsDir() && (IsConfigured(root+"/"+f.Name()) == nil) {
			chains[f.Name()] = true
		}
	}
	return chains
}

//----------------------------------------------------------------------------------------

func writeToml(path string,file string,data interface{}) error {
	p := path+"/"+file
	if fileExists(p) {
		return mkErr(path+" already exists")
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	err = enc.Encode(data);
	return err
}

func writeFile(path string,file string,data []byte) error {
	p := path+"/"+file
	if fileExists(p) {
		return mkErr(path+" already exists")
	}
	f, err := os.Create(p)
	if err != nil {return err}

	defer f.Close()
	l,err := f.Write(data)
	if (err != nil) {return err}

	if (l != len(data)) {return mkErr("unable to write all data")}
	f.Sync()
	return err
}

func readFile(path string,file string) (data []byte, err error) {
	p := path+"/"+file
	data, err = ioutil.ReadFile(p)
	return data,err
}

func mkErr(err string) error {
	return errors.New("holochain: "+err)
}

func dirExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil &&  info.Mode().IsDir();
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {return false}
	return info.Mode().IsRegular();
}
