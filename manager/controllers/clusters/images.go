package clusters

import (
	"app/manager/base"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var getClusterImagesAllowedFilters = []string{
	base.SearchQuery,
	base.DataFormatQuery,
	base.RegistryQuery,
}

var getClusterImagesFilterArgs = map[string]interface{}{
	base.SortFilterArgs: base.SortArgs{
		SortableColumns: map[string]string{
			"id":       "repository.id",
			"name":     "repository.repository",
			"registry": "repository.registry",
			"version":  "repository_image.version",
		},
		DefaultSortable: []base.SortItem{{Column: "id", Desc: false}},
	},
	base.SearchQuery: base.ImagesSearch,
}

// List of distinct columns to select before applying limits and offsets.
// Result will be returned in meta section.
var getClusterImagesDistinctValues = map[string]map[string]struct{}{
	"repository.registry": {},
}

// GetClusterImagesSelect
// @Description Exposed images in cluster data
// @Description presents in response
type GetClusterImagesSelect struct {
	Repository *string `json:"name" csv:"name"`
	Registry   *string `json:"registry" csv:"registry"`
	Version    *string `json:"version" csv:"version"`
}

type GetClusterImagesResponse struct {
	Data []GetClusterImagesSelect `json:"data"`
	Meta interface{}              `json:"meta"`
}

// GetClusterImages represents /clusters/{cluster_id}/exposed_images endpoint controller.
//
// @id GetClusterImages
// @summary List of images affecting single cluster
// @security RhIdentity || BasicAuth
// @Tags clusters
// @description Endpoint returning images affecting the given single cluster
// @accept */*
// @produce json
// @Param cluster_id      path  string   true  "cluster ID"
// @Param sort            query []string false "column for sort"                                      collectionFormat(multi) collectionFormat(csv)
// @Param search          query string   false "image name/registry search"                           example(ubi8)
// @Param limit           query int      false "limit per page"                                       example(10) minimum(0) maximum(100)
// @Param offset          query int      false "page offset"                                          example(10) minimum(0)
// @Param data_format     query string   false "data section format"                                  enums(json,csv)
// @Param report          query bool     false "overrides limit and offset to return everything"
// @router /clusters/{cluster_id}/exposed_images [get]
// @success 200 {object} GetClusterImagesResponse
// @failure 400 {object} base.Error
// @failure 404 {object} base.Error "cluster does not exist"
// @failure 500 {object} base.Error
func (c *Controller) GetClusterImages(ctx *gin.Context) {
	accountID := ctx.GetInt64("account_id")
	clusterID, err := base.GetParamUUID(ctx, "cluster_id")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, base.BuildErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	exists, err := c.ClusterExists(accountID, clusterID)
	if err != nil {
		ctx.AbortWithStatusJSON(
			http.StatusInternalServerError,
			base.BuildErrorResponse(http.StatusInternalServerError, "Internal server error"),
		)
		c.Logger.Errorf("Database error: %s", err.Error())
		return
	} else if !exists {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest,
			base.BuildErrorResponse(http.StatusNotFound, "cluster does not exist"),
		)
		return
	}

	query := c.BuildClusterImagesQuery(accountID, clusterID)
	dbErr := base.DistinctValuesQuery(query, getClusterImagesDistinctValues)
	if dbErr != nil {
		ctx.AbortWithStatusJSON(
			http.StatusInternalServerError,
			base.BuildErrorResponse(http.StatusInternalServerError, "Internal server error"),
		)
		c.Logger.Errorf("Database error: %s", dbErr.Error())
		return
	}
	imageRegistries := getClusterImagesDistinctValues["repository.registry"]

	filters := base.GetRequestedFilters(ctx)
	query = c.BuildClusterImagesQuery(accountID, clusterID)

	dataRes := []GetClusterImagesSelect{}
	usedFilters, totalItems, inputErr, dbErr := base.ListQuery(query, getClusterImagesAllowedFilters, filters, getClusterImagesFilterArgs, &dataRes)
	if inputErr != nil {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest,
			base.BuildErrorResponse(http.StatusBadRequest, inputErr.Error()),
		)
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

	resp, err := base.BuildDataMetaResponse(dataRes, base.BuildMeta(usedFilters, &totalItems, nil, nil, nil, &imageRegistries), usedFilters)
	if err != nil {
		c.Logger.Errorf("Internal server error: %s", err.Error())
	}
	ctx.JSON(http.StatusOK, resp)
}

func (c *Controller) BuildClusterImagesQuery(accountID int64, clusterID uuid.UUID) *gorm.DB {
	clusterImageSubquery := c.Conn.Table("cluster_image").
		Select(`DISTINCT cluster_image.image_id`).
		Joins("JOIN image_cve ON cluster_image.image_id = image_cve.image_id").
		Joins("JOIN cluster ON cluster_image.cluster_id = cluster.id").
		Where("cluster.account_id = ? AND cluster.uuid = ?", accountID, clusterID)

	return c.Conn.Table("repository").
		Select(`repository.repository, repository.registry, COALESCE(repository_image.version, 'Unknown') AS version`).
		Joins("JOIN repository_image ON repository.id = repository_image.repository_id").
		Joins("JOIN (?) as ci_subquery ON repository_image.image_id=ci_subquery.image_id", clusterImageSubquery)
}
