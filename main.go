package main

import (
	"bufio"
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/beevik/etree"
	flag "github.com/spf13/pflag"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	NugetServer  = "https://api.nuget.org/"      //Nuget server api address
	NugetPushUrl = NugetServer + "v3/index.json" //nuget push address
	IsEncrypt    = false                         //ApiKey is  Encrypt
	DefaultApi   = "oy2hqo4rn236xmbtqwwa4eql2k2dmtxeth4ja7tvod676y"
)

var (
	config                  *Config
	PackageIdNotConfigError = errors.New("package id not config")
	iv                      = []byte("dotnetXL")
	key                     = []byte("YangXiao")
)

func main() {
	initFlag()
	if config.Des != "" {
		cryptoText, _ := DesEncryption(key, iv, []byte(config.Des))
		fmt.Println("xl apikey", base64.URLEncoding.EncodeToString(cryptoText))
		return
	}

	fmt.Println("PackageID ", config.GetPackageId())
	nugetPackageInfo, err := GetNugetPackageInfo(config.GetPackageId())
	if err != nil {
		nugetPackageInfo = DefaultNugetPackageInfo()
	}
	v := PackPackage(nugetPackageInfo)
	if config.AutoPush {
		PushPackage(v)
	}
	fmt.Println("complete")
}

//region dotnet

func PushPackage(version string) {
	abs, _ := filepath.Abs(fmt.Sprintf("./%s/%s.%s.nupkg ", config.OutputDir, config.PackageId, version))
	CmdAndChangeDirToShow("", "dotnet", "nuget", "push", abs, "-k", config.GetAppKey(), "-s", NugetPushUrl)
}

func PackPackage(info *NugetPackageInfo) string {
	p := GetMaxVersion(info)
	p.PatchIncrease()
	fmt.Println("New Version", p.Version())
	pwd, _ := os.Getwd()
	CmdAndChangeDirToShow(pwd, "dotnet", "pack", fmt.Sprintf("-p:PackageVersion=%s", p.Version()), "--configuration", config.BuildConfiguration, "--output", config.OutputDir)
	return p.Version()
}

//endregion

//region Versions

func GetMaxVersion(v *NugetPackageInfo) *PackageVersion {
	var maxPackageVersion = DefaultPackageVersion()
	for _, version := range v.Versions {
		cv := ConvertVersion(version)
		if cv.Major > maxPackageVersion.Major {
			maxPackageVersion = cv
		} else if cv.Major == maxPackageVersion.Major {
			if cv.Minor > maxPackageVersion.Minor {
				maxPackageVersion = cv
			} else if cv.Minor == maxPackageVersion.Minor {
				if cv.Patch > maxPackageVersion.Patch {
					maxPackageVersion = cv
				}
			}
		}
	}
	return maxPackageVersion
}

func ConvertVersion(versionNumberStr string) *PackageVersion {
	versions := strings.FieldsFunc(versionNumberStr, splitFunc)

	pv := DefaultPackageVersion()

	for i, version := range versions {
		if i == 0 {
			v, _ := strconv.Atoi(version)
			pv.Major = v
		}
		if i == 1 {
			v, _ := strconv.Atoi(version)
			pv.Minor = v
		}
		if i == 2 {
			v, _ := strconv.Atoi(version)
			pv.Patch = v
		}
		if i == 2 {
			pv.Suffix = version
		}
	}
	return pv
}

type PackageVersion struct {
	Major  int
	Minor  int
	Patch  int
	Suffix string
}

func DefaultPackageVersion() *PackageVersion {
	return &PackageVersion{
		Major:  1,
		Minor:  0,
		Patch:  0,
		Suffix: "",
	}
}

func (p *PackageVersion) MajorIncrease() {
	p.Major = p.Major + 1
}
func (p *PackageVersion) MinorIncrease() {
	p.Minor = p.Minor + 1
}
func (p *PackageVersion) PatchIncrease() {
	p.Patch = p.Patch + 1
}
func (p *PackageVersion) Version() string {
	return fmt.Sprintf("%d.%d.%d", p.Major, p.Minor, p.Patch)
}

//endregion

//region Config

func initFlag() {
	config = &Config{}

	//--p=456  or --patch=456 is 456
	//--p or --patch  -1
	// nil 1
	flag.IntVarP(&config.Major, "major", "x", 0, "Major Number; -1 auto increment; 0 nothing; gt 0 custom; missing is nothing")
	flag.Lookup("major").NoOptDefVal = "0"
	flag.IntVarP(&config.Minor, "minor", "y", 0, "Minor Number; 1 auto increment; 0 nothing; gt 0 custom; missing is nothing")
	flag.Lookup("minor").NoOptDefVal = "0"
	flag.IntVarP(&config.Patch, "patch", "z", -1, "Patch Number; 1 auto increment; 0 nothing; gt 0 custom; missing is auto increment")
	flag.Lookup("patch").NoOptDefVal = "-1"

	flag.BoolVarP(&config.AutoPush, "push", "p", false, "Auto Push Nuget serve")
	flag.Lookup("push").NoOptDefVal = "true"

	flag.BoolVarP(&config.NoBuild, "no-build", "n", false, "Doesn't build the project before packing")
	flag.Lookup("no-build").NoOptDefVal = "true"

	flag.StringVarP(&config.BuildConfiguration, "configuration", "c", "Debug", "Doesn't build the project before packing")
	flag.Lookup("no-build").NoOptDefVal = "Debug"

	flag.StringVarP(&config.ApiKey, "api-key", "k", "", "push to server apikey")
	flag.Lookup("api-key").NoOptDefVal = DefaultApi

	flag.StringVarP(&config.ProjectFile, "project", "f", "", "csproj file path; nil search current directory")
	flag.StringVarP(&config.PackageId, "packageId", "i", "", "csproj file path; nil search current directory")
	flag.StringVarP(&config.OutputDir, "output", "o", "nupkgs", "Places the built packages in the directory specified.")

	flag.StringVar(&config.Des, "des", "", "Use Des Encryption a apiKey")

	flag.Parse()

}

type Config struct {
	ApiKey             string //ApiKey
	ProjectFile        string //csproj file path nil search ./*csproj first
	Major              int    // -1 auto increment; 0 nothing; gt 0 custom
	Minor              int    // -1 auto increment; 0 nothing; gt 0 custom
	Patch              int    // -1 auto increment; 0 nothing; gt 0 custom
	Suffix             string // TODO:   beta preview
	PackageId          string // nuget packageId nil find node from ProjectFile
	AutoPush           bool   // Auto Push Nuget server
	NoBuild            bool   // Doesn't build the project before packing
	OutputDir          string // Places the built packages in the directory specified.
	BuildConfiguration string // Defines the build configuration. The default for most projects is Debug, but you can override the build configuration settings in your project.
	Des                string
}

func (c *Config) GetPackageId() string {
	if c.PackageId == "" {
		c.PackageId = FindPackageId(c.ProjectFile)
	}
	return c.PackageId
}

func (c *Config) GetAppKey() string {
	if IsEncrypt {
		decodeString, _ := base64.URLEncoding.DecodeString(c.ApiKey)
		key, _ = DesDecryption(key, iv, []byte(decodeString))
		return string(key)
	}
	return c.ApiKey
}

//endregion

//region Nuget pkg

//GetNugetPackageInfo 获取包的版本号
func GetNugetPackageInfo(packageId string) (*NugetPackageInfo, error) {
	cli := http.Client{}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%sv3-flatcontainer/%s/index.json", NugetServer, strings.ToLower(packageId)), nil)
	if err != nil {
		fmt.Errorf("Nuget Request Error  %s", err.Error())
		return nil, err
	}
	req.Header.Add("X-NuGet-ApiKey", config.GetAppKey())
	rep, err := cli.Do(req)
	if err != nil {
		fmt.Errorf("Nuget Response Error  %s", err.Error())
		return nil, err
	}
	var result NugetPackageInfo
	err = json.NewDecoder(rep.Body).Decode(&result)
	if err != nil {
		fmt.Errorf("Nuget Json Decoder Error  %s", err.Error())
		return nil, err
	}
	return &result, nil
}

//NugetPackageInfo Nuget package Info
type NugetPackageInfo struct {
	Versions []string `json:"versions"`
}

func DefaultNugetPackageInfo() *NugetPackageInfo {
	return &NugetPackageInfo{Versions: []string{"0.0.0"}}
}

//endregion

//region pkg

func FindPackageId(projectFileName string) string {
	if projectFileName == "" {
		projectFileName = FindProjectFile()
		if projectFileName == "" {
			panic(PackageIdNotConfigError)
		}
	}
	fmt.Println("project file ", projectFileName)
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(projectFileName); err != nil {
		panic(err)
	}
	root := doc.SelectElement("Project")
	for _, element := range root.SelectElements("PropertyGroup") {
		if packageIdElement := element.SelectElement("PackageId"); packageIdElement != nil {
			return packageIdElement.Text()
		}
	}
	panic(PackageIdNotConfigError)
}

func FindProjectFile() string {
	pwd, _ := os.Getwd()
	fmt.Println("search project file in ", pwd)
	files, _ := ioutil.ReadDir(pwd)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".csproj") {
			return file.Name()
		}
	}
	return ""
}

func splitFunc(r rune) bool {
	return r == '.' || r == '-'
}

func DesEncryption(key, iv, plainText []byte) ([]byte, error) {

	block, err := des.NewCipher(key)

	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	origData := PKCS5Padding(plainText, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv)
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)
	return cryted, nil
}

func DesDecryption(key, iv, cipherText []byte) ([]byte, error) {

	block, err := des.NewCipher(key)

	if err != nil {
		return nil, err
	}

	blockMode := cipher.NewCBCDecrypter(block, iv)
	origData := make([]byte, len(cipherText))
	blockMode.CryptBlocks(origData, cipherText)
	origData = PKCS5UnPadding(origData)
	return origData, nil
}

func PKCS5Padding(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padtext...)
}

func PKCS5UnPadding(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])
	return src[:(length - unpadding)]
}

//endregion

//region cmd

type Charset string

const (
	UTF8    = Charset("UTF-8")
	GB18030 = Charset("GB18030")
)

func CmdAndChangeDirToShow(dir string, commandName string, params ...string) error {
	cmd := exec.Command(commandName, params...)
	cmd.Stderr = os.Stderr
	if dir != "" {
		cmd.Dir = dir
	}
	return execCmd(cmd)
}

func ExecShellString(shellString string, dir string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", shellString)
	} else {
		cmd = exec.Command("/bin/bash", "-c", shellString)
	}
	cmd.Stderr = os.Stdout
	cmd.Dir = dir
	return execCmd(cmd)
}

func execCmd(cmd *exec.Cmd) (err error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Errorf("cmd.StdoutPipe: %s", err.Error())
		return
	}
	err = cmd.Start()
	if err != nil {
		fmt.Errorf("cmd.Start: %s", err)
		return
	}
	reader := bufio.NewReader(stdout)
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			fmt.Errorf("Exec: %s End; %s", cmd.Dir, cmd.Args, err2)
			break
		}
		fmt.Println(convertByte2String(line, GB18030))
	}
	err = cmd.Wait()
	return
}

func convertByte2String(strOld string, charset Charset) string {
	var str string
	switch charset {
	case GB18030:
		str, _ = simplifiedchinese.GB18030.NewDecoder().String(strOld)
	case UTF8:
		fallthrough
	default:
		str = strOld
	}
	return str
}

//endregion
