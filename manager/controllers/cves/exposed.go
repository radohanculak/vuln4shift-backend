package cves

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"app/base/utils"
	"app/manager/amsclient"
	"app/manager/base"
)

// GetExposedClustersSelect
// @Description CVE exposed clusters data
// @Description presents in response
type GetExposedClustersSelect struct {
	UUID        string     `json:"id" csv:"id"`
	DisplayName string     `json:"display_name" csv:"display_name"`
	Status      string     `json:"status" csv:"status"`
	Type        string     `json:"type" csv:"type"`
	Version     string     `json:"version" csv:"version"`
	Provider    string     `json:"provider" csv:"provider"`
	LastSeen    *time.Time `json:"last_seen" csv:"last_seen"`
}

type GetExposedClustersResponse struct {
	Data []GetExposedClustersSelect `json:"data"`
	Meta interface{}                `json:"meta"`
}

var (
	getExposedClustersAllowedFilters = []string{
		base.SearchQuery,
		base.DataFormatQuery,
		base.ProviderQuery,
		base.StatusQuery,
		base.VersionQuery,
	}

	getExposedClustersFilterArgs = map[string]interface{}{
		base.SortFilterArgs: base.SortArgs{
			SortableColumns: map[string]string{
				"id":           "cluster.id",
				"status":       "cluster.status",
				"version":      "cluster.version",
				"provider":     "cluster.provider",
				"uuid":         "cluster.uuid",
				"last_seen":    "cluster.last_seen",
				"display_name": "cluster.display_name",
				"type":         "cluster.type"},
			DefaultSortable: []base.SortItem{{Column: "id", Desc: false}},
		},
		base.SearchQuery: base.ExposedClustersSearch,
	}
)

// GetExposedClusters represents exposed clusters endpoint controller.
//
// @id GetExposedClusters
// @summary List of exposed clusters for CVE
// @security RhIdentity || BasicAuth
// @Tags cves
// @description Endpoint return exposed clusters for given CVE
// @accept */*
// @produce json
// @Param cve_name    path  string   true  "CVE name"
// @Param sort        query []string false "column for sort"      collectionFormat(multi) collectionFormat(csv)
// @Param search      query string   false "cluster search"       example(00000000-0000-0000-0000-000000000022)
// @Param limit       query int      false "limit per page"       example(10) minimum(0) maximum(100)
// @Param offset      query int      false "page offset"          example(10) minimum(0)
// @Param data_format query string   false "data section format"  enums(json,csv)
// @Param report      query bool     false "overrides limit and offset to return everything"
// @router /cves/{cve_name}/exposed_clusters [get]
// @success 200 {object} GetExposedClustersResponse
// @failure 400 {object} base.Error
// @failure 404 {object} base.Error "{cve_name} not found"
// @failure 500 {object} base.Error
func (c *Controller) GetExposedClusters(ctx *gin.Context) {
	accountID := ctx.GetInt64("account_id")
	orgID := ctx.GetString("org_id")
	cveName := ctx.Param("cve_name")
	filters := base.GetRequestedFilters(ctx)

	// Check if CVE exists first
	query := c.BuildCveDetailsQuery(cveName)
	var cveDetails GetCveDetailsSelect
	result := query.First(&cveDetails)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(
				http.StatusNotFound,
				base.BuildErrorResponse(http.StatusNotFound, fmt.Sprintf("%s not found", cveName)),
			)
			return
		}
		ctx.AbortWithStatusJSON(
			http.StatusInternalServerError,
			base.BuildErrorResponse(http.StatusInternalServerError, "Internal server error"),
		)
		c.Logger.Errorf("Database error: %s", result.Error)
		return
	}

	clusterIDs, clusterStatuses, clusterVersions, clusterProviders, err := amsclient.DBFetchClusterDetails(c.Conn, c.AMSClient, accountID, orgID, utils.Cfg.AmsEnabled, &cveName)
	if err != nil {
		ctx.AbortWithStatusJSON(
			http.StatusInternalServerError,
			base.BuildErrorResponse(http.StatusInternalServerError, "Internal server error"),
		)
		c.Logger.Errorf("Error fetching AMS data: %s", err.Error())
		return
	}

	query = c.BuildExposedClustersQuery(cveName, accountID, clusterIDs)

	exposedClusters := []GetExposedClustersSelect{}
	usedFilters, totalItems, inputErr, dbErr := base.ListQuery(query, getExposedClustersAllowedFilters, filters, getExposedClustersFilterArgs, &exposedClusters)
	if inputErr != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, base.BuildErrorResponse(http.StatusBadRequest, inputErr.Error()))
		return
	}
	if dbErr != nil {
		ctx.AbortWithStatusJSON(
			http.StatusInternalServerError,
			base.BuildErrorResponse(http.StatusInternalServerError, "Internal server error"),
		)
		c.Logger.Errorf("Database error: %s", dbErr.Error())
		return
	}

	resp, err := base.BuildDataMetaResponse(exposedClusters, base.BuildMeta(usedFilters, &totalItems, &clusterStatuses, &clusterVersions, &clusterProviders, nil), usedFilters)
	if err != nil {
		c.Logger.Errorf("Internal server error: %s", err.Error())
	}
	ctx.JSON(http.StatusOK, resp)
}

func (c *Controller) BuildExposedClustersQuery(cveName string, accountID int64, clusterIDs []string) *gorm.DB {
	query := c.Conn.Table("cluster").
		Select(`cluster.uuid,
				COALESCE(cluster.display_name, cluster.uuid::text) as display_name,
				COALESCE(cluster.status, 'N/A') as status,
				COALESCE(cluster.type, 'N/A') as type,
				COALESCE(cluster.version, 'N/A') as version,
				COALESCE(cluster.provider, 'N/A') as provider,
				cluster.last_seen,
				COUNT(DISTINCT cluster_image.image_id) as images_exposed`).
		Joins("JOIN cluster_image ON cluster.id = cluster_image.cluster_id").
		Joins("JOIN image_cve ON cluster_image.image_id = image_cve.image_id").
		Joins("JOIN cve ON image_cve.cve_id = cve.id").
		Group("cluster.id").
		Where("cve.name = ?", cveName).
		Where("cluster.account_id = ?", accountID)

	if utils.Cfg.AmsEnabled {
		query = query.Where("cluster.uuid IN ?", clusterIDs)
	}

	return query
}
