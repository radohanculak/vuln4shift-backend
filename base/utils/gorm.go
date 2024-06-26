package utils

import (
	"database/sql/driver"
	"sort"
)

// ByteArrayBool sets to true if the array is not null or empty during the GORM scan, else false.
type ByteArrayBool bool

var lowPrioTags = map[string]bool{
	"latest": true,
}

func (s *ByteArrayBool) Scan(value interface{}) error {
	if value == nil {
		*s = false
	}

	res := value.([]byte)
	if len(res) > 0 {
		*s = true
	} else {
		*s = false
	}

	return nil
}

func (s ByteArrayBool) Value() (driver.Value, error) {
	if s {
		return "true", nil
	}
	return "false", nil
}

func SortTags(tags *[]string) {
	if tags != nil {
		// Sort slice with tags primarily by length
		// If the length is same, sort alphabetically
		// If the tag is one of the low prio tags, intercept, and put it to the end
		// (no additional sort is applied if there are more low prio tags)
		sort.Slice(*tags, func(i, j int) bool {
			if _, found := lowPrioTags[(*tags)[i]]; found {
				return false
			}
			if _, found := lowPrioTags[(*tags)[j]]; found {
				return true
			}
			lenI, lenJ := len((*tags)[i]), len((*tags)[j])
			if lenI != lenJ {
				return lenI > lenJ
			}
			return (*tags)[i] < (*tags)[j]
		})
	}
}
