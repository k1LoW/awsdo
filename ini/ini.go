package ini

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

type Ini struct {
	configIni *ini.File
	credsIni  *ini.File
}

func (i *Ini) GetKey(profile, key string) string {
	switch {
	case i.credsIni.Section(profile).Key(key).String() != "":
		return i.credsIni.Section(profile).Key(key).String()
	case i.credsIni.Section(fmt.Sprintf("profile %s", profile)).Key(key).String() != "":
		return i.credsIni.Section(fmt.Sprintf("profile %s", profile)).Key(key).String()
	case i.configIni.Section(profile).Key(key).String() != "":
		return i.configIni.Section(profile).Key(key).String()
	case i.configIni.Section(fmt.Sprintf("profile %s", profile)).Key(key).String() != "":
		return i.configIni.Section(fmt.Sprintf("profile %s", profile)).Key(key).String()
	case i.credsIni.Section("default").Key(key).String() != "":
		return i.credsIni.Section("default").Key(key).String()
	case i.configIni.Section("default").Key(key).String() != "":
		return i.configIni.Section("default").Key(key).String()
	}
	return ""
}

func New() (*Ini, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configIniPath := os.Getenv("AWS_CONFIG_FILE")
	if configIniPath == "" {
		configIniPath = filepath.Join(home, ".aws", "config")
	}
	credsIniPath := filepath.Join(home, ".aws", "credentials")

	configIni, err := ini.Load(configIniPath)
	if err != nil {
		return nil, err
	}

	credsIni, err := ini.Load(credsIniPath)
	if err != nil {
		return nil, err
	}

	return &Ini{
		configIni: configIni,
		credsIni:  credsIni,
	}, nil
}
