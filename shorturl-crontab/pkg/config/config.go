package config

import (
	"github.com/spf13/viper"
	"log"
)

// Config 结构体定义了应用程序的配置项，包括HTTP、MySQL、Redis和日志相关的配置。
type Config struct {
	Mysql struct {
		DSN         string
		MaxLifeTime int
		MaxOpenConn int
		MaxIdleConn int
	}
	Redis struct {
		Host string
		Port int
		Pwd  string `mapstructure:"pwd"`
	}
	Log struct {
		Level   string
		LogPath string `mapstructure:"logPath"`
	} `mapstructure:"log"`
}

var conf *Config

// InitConfig 初始化应用程序的配置。
// 该函数通过读取指定路径的配置文件，并将其解析为Config结构体。
// 参数：
//   - filePath: 配置文件的路径。
//   - typ: 配置文件的类型（可选），如果不提供，则根据文件扩展名自动推断。
//
// 返回值：无
// 如果配置文件读取或解析失败，函数将记录错误并终止程序。
func InitConfig(filePath string, typ ...string) {
	v := viper.New()
	v.SetConfigFile(filePath)
	if len(typ) > 0 {
		v.SetConfigType(typ[0])
	}
	err := v.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}
	conf = &Config{}
	err = v.Unmarshal(conf)
	if err != nil {
		log.Fatal(err)
	}
}

// GetConfig 返回当前应用程序的配置。
// 返回值：
//   - *Config: 当前应用程序的配置结构体指针。
func GetConfig() *Config {
	return conf
}
