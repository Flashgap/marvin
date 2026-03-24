package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Flashgap/logrus"
)

// PrintConfig prints a config struct in a table, using reflection to get the field
// config name, the environment variable name, and the value, and whether it is a secret or not to hide it.
func PrintConfig(c any) {
	type result struct {
		category   string
		configName string
		envName    string
		value      string
	}
	type results struct {
		r                []result
		maxCategoryLen   int
		maxConfigNameLen int
		maxEnvNameLen    int
		maxValueLen      int
	}
	agg := func(resultsToAggregate ...results) results {
		var res results
		for _, r := range resultsToAggregate {
			res.r = append(res.r, r.r...)
			if r.maxCategoryLen > res.maxCategoryLen {
				res.maxCategoryLen = r.maxCategoryLen
			}
			if r.maxConfigNameLen > res.maxConfigNameLen {
				res.maxConfigNameLen = r.maxConfigNameLen
			}
			if r.maxEnvNameLen > res.maxEnvNameLen {
				res.maxEnvNameLen = r.maxEnvNameLen
			}
			if r.maxValueLen > res.maxValueLen {
				res.maxValueLen = r.maxValueLen
			}
		}
		return res
	}
	app := func(r results, category, configName, envName, value string) results {
		r.r = append(r.r, result{
			category:   category,
			configName: configName,
			envName:    envName,
			value:      value,
		})
		if l := len(category); l > r.maxCategoryLen {
			r.maxCategoryLen = l
		}
		if l := len(configName); l > r.maxConfigNameLen {
			r.maxConfigNameLen = l
		}
		if l := len(envName); l > r.maxEnvNameLen {
			r.maxEnvNameLen = l
		}
		if l := len(value); l > r.maxValueLen {
			r.maxValueLen = l
		}
		return r
	}

	var rec func(c any) results
	rec = func(c any) results {
		var subResults []results
		var directResults results

		v := reflect.ValueOf(c)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		if v.Kind() != reflect.Struct {
			logrus.Errorln("printConfig only accepts structs; got", v.Kind())
			return results{}
		}

		category := v.Type().Name()

		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if field.Type.Kind() == reflect.Struct {
				subResults = append(subResults, rec(v.Field(i).Interface()))
			} else {
				secretTag := field.Tag.Get("secret")
				configName := field.Name
				envName := field.Tag.Get("envconfig")
				ignored := field.Tag.Get("ignored")
				if ignored == "" && envName == "" {
					envName = strings.ToUpper(configName)
				}
				var value any
				if secretTag != "" && strings.ToLower(secretTag) != "false" && strings.ToLower(secretTag) != "no" {
					zero := reflect.Zero(field.Type).Interface()
					if reflect.DeepEqual(zero, v.Field(i).Interface()) {
						value = "{UNSET SECRET}"
					} else {
						value = "******** (secret)"
					}
				} else {
					value = v.Field(i).Interface()
				}

				directResults = app(directResults, category, configName, envName, fmt.Sprintf("%v", value))
			}
		}

		return agg(append([]results{directResults}, subResults...)...)
	}

	res := rec(c)
	var previousCategory string
	headers := []result{
		{
			category:   "CATEGORY",
			configName: "CONFIG NAME",
			envName:    "ENV NAME",
			value:      "VALUE",
		},
	}
	logrus.Infof("Config has been loaded")
	toRange := append(headers, res.r...) //nolint:gocritic // appendAssign: intentional — headers must stay unmodified; toRange is a read-only merged view
	for i, r := range toRange {
		category := "  ..."
		if r.category != previousCategory {
			category = r.category
			previousCategory = r.category
		}
		logrus.Infof("%-*s | %-*s | %-*s | %s", res.maxCategoryLen, category, res.maxConfigNameLen, r.configName, res.maxEnvNameLen, r.envName, r.value)
		if i == 0 {
			logrus.Info(strings.Repeat("-", res.maxCategoryLen+res.maxConfigNameLen+res.maxEnvNameLen+res.maxValueLen+6))
		}
	}
}
