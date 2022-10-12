package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"reflect"
	"strings"
)

type Config struct {
	Port    string `mapstructure:"PORT" validate:"required"`
	GcsFile string `mapstructure:"GCS_FILE" validate:"required"`
	Debug   bool   `mapstructure:"DEBUG"`
}

func bindEnvs(iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			continue
		}
		switch v.Kind() {
		case reflect.Struct:
			bindEnvs(v.Interface(), append(parts, tv)...)
		default:
			_ = viper.BindEnv(strings.Join(append(parts, tv), "."))
		}
	}
}

func Load() (config Config, err error) {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// I hate this, but it works.
	// This is to not require a config file to unmarshal Envs in a struct
	// https://github.com/spf13/viper/issues/188#issuecomment-399884438
	config = Config{}
	bindEnvs(config)

	err = viper.Unmarshal(&config)
	validate := validator.New()

	if err := validate.Struct(&config); err != nil {
		log.Fatal().Err(err).Msg("Missing required environment variables.")
	}

	return
}
