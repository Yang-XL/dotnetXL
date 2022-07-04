package main

import (
	"bufio"
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
	"runtime"
	"strconv"
	"strings"
)

var config Config

func main() {
	initFlag()
	fmt.Println("PackageID ", config.GetPackageId())
	nugetPackageInfo := GetNugetPackageInfo(config.GetPackageId())
	p := GetMaxVersion(nugetPackageInfo)
	p.PatchIncrease()
	fmt.Println(p.Version())
	packageVersionParam := fmt.Sprintf("-p:PackageVersion=%s", p.Version())
	CmdAndChangeDirToShow("", "dotnet", "pack", packageVersionParam)
	var e string
	fmt.Scanln(&e)
}

//region dotnet pack

func FindPackageId(projectFileName string) string {
	if projectFileName == "" {
		projectFileName = FindProjectFile()
		if projectFileName == "" {
			panic(errors.New("project file not found"))
		}
	}

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
	panic(errors.New("package id not config"))
}

func FindProjectFile() string {
	files, _ := ioutil.ReadDir("./")
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".csproj") {
			return file.Name()
		}
	}
	return ""
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
	config = Config{AppKey: "oy2hqo4rn236xmbtqwwa4eql2k2dmtxeth4ja7tvod676y"}

	//--p=456  or --patch=456 is 456
	//--p or --patch  -1
	// nil 1
	flag.IntVarP(&config.Major, "major", "x", 0, "Major Number; -1 auto increment; 0 nothing; gt 0 custom; missing is nothing")
	flag.Lookup("major").NoOptDefVal = "0"
	flag.IntVarP(&config.Minor, "minor", "y", 0, "Minor Number; 1 auto increment; 0 nothing; gt 0 custom; missing is nothing")
	flag.Lookup("minor").NoOptDefVal = "0"
	flag.IntVarP(&config.Patch, "patch", "z", -1, "Patch Number; 1 auto increment; 0 nothing; gt 0 custom; missing is auto increment")
	flag.Lookup("patch").NoOptDefVal = "-1"

	flag.StringVarP(&config.ProjectFile, "project", "p", "", "csproj file path; nil search current directory")
	flag.Lookup("project").NoOptDefVal = ""
	flag.StringVarP(&config.PackageId, "packageId", "i", "", "csproj file path; nil search current directory")
	flag.Lookup("packageId").NoOptDefVal = ""
	flag.Parse()
}

type Config struct {
	AppKey      string //AppKey 推送使用
	ProjectFile string //csproj file path nil search ./*csproj first
	Major       int    // -1 auto increment; 0 nothing; gt 0 custom
	Minor       int    // -1 auto increment; 0 nothing; gt 0 custom
	Patch       int    // -1 auto increment; 0 nothing; gt 0 custom
	Suffix      string // 暂不使用   beta preview
	PackageId   string // nuget packageId nil find node from ProjectFile
}

func (c *Config) GetPackageId() string {
	if c.PackageId == "" {
		return FindPackageId(c.ProjectFile)
	}
	return c.PackageId
}

func (c *Config) GetAppKey() string {
	return c.AppKey
}

//endregion

//region Nuget pkg

//GetNugetPackageInfo 获取包的版本号
func GetNugetPackageInfo(packageId string) *NugetPackageInfo {
	cli := http.Client{}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.nuget.org/v3-flatcontainer/%s/index.json", strings.ToLower(packageId)), nil)
	if err != nil {
		fmt.Errorf("Nuget Request Error  %s", err.Error())
		return nil
	}
	req.Header.Add("X-NuGet-ApiKey", config.GetAppKey())
	rep, err := cli.Do(req)
	if err != nil {
		fmt.Errorf("Nuget Response Error  %s", err.Error())
		return nil
	}
	var result NugetPackageInfo
	err = json.NewDecoder(rep.Body).Decode(&result)
	if err != nil {
		fmt.Errorf("Nuget Json Decoder Error  %s", err.Error())
		return nil
	}
	return &result
}

//NugetPackageInfo Nuget package Info
type NugetPackageInfo struct {
	Versions []string `json:"versions"`
}

//endregion

//region pkg

func splitFunc(r rune) bool {
	return r == '.' || r == '-'
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
	fmt.Println(cmd.Path, cmd.Args)
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
