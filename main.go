package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var config Config

func main() {
	config = Config{AppKey: "oy2hqo4rn236xmbtqwwa4eql2k2dmtxeth4ja7tvod676y"}
	nugetPackageInfo := GetNugetPackageInfo("Yxl.Dal")

	p := GetMaxVersion(nugetPackageInfo)

	fmt.Println(p.Major, p.Minor, p.Patch)
	p.PatchIncrease()
	fmt.Println(p.Major, p.Minor, p.Patch)

}




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

type Config struct {
	AppKey string
}

func (c *Config) GetAppKey() string {
	return c.AppKey
}

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
