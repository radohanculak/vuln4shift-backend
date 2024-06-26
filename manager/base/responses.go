package base

import (
	"errors"
	"sort"

	"github.com/gocarina/gocsv"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type Error struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

type DataMetaResponse struct {
	Data interface{} `json:"data"`
	Meta interface{} `json:"meta"`
}

func BuildErrorResponse(status int, detail string) Error {
	return Error{Error: ErrorDetail{Detail: detail, Status: status}}
}

func BuildDataMetaResponse(data interface{}, meta interface{}, filters map[string]Filter) (DataMetaResponse, error) {
	if filter, exists := filters[DataFormatQuery]; exists {
		dataFormat, ok := filter.(*DataFormat)
		if !ok {
			return DataMetaResponse{}, errors.New("Invalid data format filter")
		}

		switch dataFormat.Value {
		case CSVFormat:
			data, err := gocsv.MarshalString(data)
			if err != nil {
				return DataMetaResponse{}, err
			}
			return DataMetaResponse{data, meta}, nil
		}
	}
	return DataMetaResponse{data, meta}, nil
}

// BuildMeta creates Meta section in response from requested filters
// result is map with query args and their raw values
func BuildMeta(requestedFilters map[string]Filter, totalItems *int64, clusterStatuses, clusterVersions, clusterProviders, imageRegistries *map[string]struct{}) map[string]interface{} {
	meta := make(map[string]interface{})
	for _, filter := range requestedFilters {
		meta[filter.RawQueryName()] = filter.RawQueryVal()
	}
	if totalItems != nil {
		meta["total_items"] = *totalItems
	}
	if clusterStatuses != nil {
		var statuses []string
		for status := range *clusterStatuses {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)
		meta["cluster_statuses_all"] = statuses
	}
	if clusterVersions != nil {
		var versions []string
		for version := range *clusterVersions {
			versions = append(versions, version)
		}
		cl := collate.New(language.English, collate.Numeric)
		cl.SortStrings(versions)
		meta["cluster_versions_all"] = versions
	}
	if clusterProviders != nil {
		var providers []string
		for provider := range *clusterProviders {
			providers = append(providers, provider)
		}
		sort.Strings(providers)
		meta["cluster_providers_all"] = providers
	}
	if imageRegistries != nil {
		var registries []string
		for registry := range *imageRegistries {
			registries = append(registries, registry)
		}
		sort.Strings(registries)
		meta["image_registries_all"] = registries
	}
	return meta
}
