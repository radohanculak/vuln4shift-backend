package base

import (
	"app/base/models"
	"app/base/utils"

	"gorm.io/gorm"
)

// ListQuery applies filters, queries and gets total count for listable endpoint
func ListQuery(tx *gorm.DB, allowedFilters []string, filters map[string]Filter,
	filterArgs map[string]interface{}, result interface{}) (usedFilters map[string]Filter, totalItems int64, inputError error, dbError error) {
	usedFilters = make(map[string]Filter)
	uf, inputError := ApplyFilters(tx, allowedFilters, filters, filterArgs)
	if inputError != nil {
		return
	}
	usedFilters = utils.CopyMap(uf, usedFilters)

	res := tx.Count(&totalItems)
	if res.Error != nil {
		dbError = res.Error
		return
	}

	// report needs to be always after the limit & offset to reset them
	spqFilters := []string{SortQuery, LimitQuery, OffsetQuery, ReportQuery}
	uf, inputError = ApplyFilters(tx, spqFilters, filters, filterArgs)
	if inputError != nil {
		return
	}
	usedFilters = utils.CopyMap(uf, usedFilters)

	if result == nil {
		return
	}

	res = tx.Find(result)
	if res.Error != nil {
		dbError = res.Error
		return
	}
	return
}

// This selects distinct values of specified columns, without any filters, offsets and limits
func DistinctValuesQuery(tx *gorm.DB, distinctValues map[string]map[string]struct{}) (dbError error) {
	for column, values := range distinctValues {
		results := []models.Repository{}
		res := tx.Distinct(column).Find(&results)
		if res.Error != nil {
			dbError = res.Error
			return
		}
		for _, repository := range results {
			values[repository.Registry] = struct{}{}
		}
	}
	return
}
