package api

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"pwd-checker/internal/util"
	"reflect"
	"strings"
)

type Config struct {
	Port    string `mapstructure:"PORT" validate:"required"`
	GcsFile string `mapstructure:"GCS_FILE" validate:"required"`
	SelfTLS bool   `mapstructure:"SELF_TLS" validate:"required_without_all=TLSCert TLSKey"`
	TLSCert string `mapstructure:"TLS_CERT" validate:"required_if=SelfTLS false,required_with=TLSKey"`
	TLSKey  string `mapstructure:"TLS_KEY" validate:"required_if=SelfTLS false,required_with=TLSCert"`
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

func msgForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "required_without_all":
		return fmt.Sprintf("This field is required if fields [%s] are missing", util.ToScreamingSnakeCase(fe.Param()))
	case "required_if":
		return fmt.Sprintf("This field is required if %s", util.ToScreamingSnakeCase(fe.Param()))
	case "required_with":
		return fmt.Sprintf("This is field requires the presence of %s", util.ToScreamingSnakeCase(fe.Param()))
	}
	return fe.Error() // default error
}

func LoadConfig() (config Config, err error) {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// I hate this, but it works.
	// This is to not require a config file to unmarshal Envs in a struct
	// https://github.com/spf13/viper/issues/188#issuecomment-399884438
	config = Config{}
	bindEnvs(config)

	err = viper.Unmarshal(&config)
	validate := validator.New()

	if err = validate.Struct(&config); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			var msgs []string
			for _, fe := range ve {
				msgs = append(msgs, fmt.Sprintf("%s: %s", util.ToScreamingSnakeCase(fe.Field()), msgForTag(fe)))
			}

			log.Fatal().Msgf("%s", strings.Join(msgs, ". "))
		} else {
			log.Fatal().Err(err).Msg("missing validating configuration from environment.")
		}
	}

	return
}
