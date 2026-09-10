package clients

import (
	"cmp"
	"context"
	"fmt"
	"func/pkg/clients/collection"
	"func/pkg/logging"
	"func/pkg/results"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"slices"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/analytics"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/integration"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"
)

const (
	query string = `query autonomousdatabase, analyticsinstance resources
	where lifeCycleState = 'RUNNING' || lifeCycleState = 'STOPPED' || lifeCycleState = 'AVAILABLE'
	|| lifeCycleState = 'ACTIVE' || lifeCycleState = 'INACTIVE'`
	dbQuery string = `query dbsystem resources where lifeCycleState = 'AVAILABLE' &&
	licenseType = 'LICENSE_INCLUDED'`
	integrationQuery string = `query integrationinstance resources
	where isbyol = 'false' && lifeCycleState = 'ACTIVE'`
	clusterQuery string = `query cloudvmcluster, vmcluster, autonomousvmcluster, cloudautonomousvmcluster resources
	where lifeCycleState = 'AVAILABLE'`

	searchLimit         = 1000
	maxConcurrentUpdate = 10
)

var logger *slog.Logger = logging.NewLogger(os.Getenv("LOG_LEVEL"))

type RegionalClient struct {
	AnalyticsClient   analytics.AnalyticsClient
	DatabaseClient    database.DatabaseClient
	SearchClient      resourcesearch.ResourceSearchClient
	IntegrationClient integration.IntegrationInstanceClient
}

type ClientBundle map[string]RegionalClient

func NewRegionalClient(p common.ConfigurationProvider) (RegionalClient, error) {
	ac, err := analytics.NewAnalyticsClientWithConfigurationProvider(p)
	if err != nil {
		return RegionalClient{}, fmt.Errorf("create analytics client: %w", err)
	}

	dc, err := database.NewDatabaseClientWithConfigurationProvider(p)
	if err != nil {
		return RegionalClient{}, fmt.Errorf("create database client: %w", err)
	}

	sc, err := resourcesearch.NewResourceSearchClientWithConfigurationProvider(p)
	if err != nil {
		return RegionalClient{}, fmt.Errorf("create resource search client: %w", err)
	}

	ic, err := integration.NewIntegrationInstanceClientWithConfigurationProvider(p)
	if err != nil {
		return RegionalClient{}, fmt.Errorf("create integration client: %w", err)
	}

	return RegionalClient{
		AnalyticsClient:   ac,
		DatabaseClient:    dc,
		SearchClient:      sc,
		IntegrationClient: ic,
	}, nil
}

func NewClientBundle(regions []identity.RegionSubscription) (ClientBundle, []error) {
	logger.Debug("making client bundle", "regions", len(regions))

	cb := make(ClientBundle)
	var errs []error

	for _, r := range regions {
		if r.RegionName == nil || *r.RegionName == "" {
			errs = append(errs, fmt.Errorf("region subscription missing region name"))
			continue
		}

		regionName := *r.RegionName
		newProvider, err := auth.ResourcePrincipalConfigurationProviderForRegion(common.StringToRegion(regionName))
		if err != nil {
			errs = append(errs, fmt.Errorf("create provider for region %s: %w", regionName, err))
			continue
		}

		client, err := NewRegionalClient(newProvider)
		if err != nil {
			errs = append(errs, fmt.Errorf("create clients for region %s: %w", regionName, err))
			continue
		}

		cb[regionName] = client
	}

	logger.Debug("client bundle assembled", "regions", len(cb), "failures", len(errs))
	return cb, errs
}

func (c ClientBundle) ProcessCollection(ctx context.Context) *results.Result {
	logger.Debug("processing resource collection")

	var wg sync.WaitGroup
	sc := collection.NewSearchCollection()
	result := results.NewResult()

	logger.Debug(
		"searching regions",
		"regions", len(c),
		"queries", []string{query, dbQuery, integrationQuery, clusterQuery},
	)
	for _, region := range sortedKeys(c) {
		client := c[region]
		wg.Add(1)
		go func(client RegionalClient, region string) {
			defer wg.Done()

			rc, err := client.Search(ctx, region)
			if err != nil {
				logger.Error("search failed", "region", region, "error", err)
				result.AddFailures(1)
				return
			}

			logger.Info("search complete", "region", region, "items_found", len(rc.Items))

			sc.Lock()
			defer sc.Unlock()

			sc.Items[region] = rc
			result.AddItemsFound(len(rc.Items))
		}(client, region)
	}

	wg.Wait()

	if logger.Enabled(ctx, slog.LevelDebug) {
		logger.Debug("items returned", "items", sc.JsonEncode())
	}

	type updateJob struct {
		item         resourcesearch.ResourceSummary
		region       string
		resourceType string
		resourceID   string
	}

	jobs := make(chan updateJob)
	var updateWG sync.WaitGroup
	for range maxConcurrentUpdate {
		updateWG.Add(1)
		go func() {
			defer updateWG.Done()
			for job := range jobs {
				changed, err := c[job.region].handleResource(ctx, job.item, job.region, job.resourceType, job.resourceID)
				if err != nil {
					logger.Error(
						"resource update failed",
						"region", job.region,
						"resource_type", job.resourceType,
						"resource_id", job.resourceID,
						"error", err,
					)
					result.AddFailures(1)
					continue
				}
				if changed {
					result.AddChanges(1)
				}
			}
		}()
	}

	for _, region := range sortedKeys(sc.Items) {
		items := sc.Items[region]
		for _, item := range items.Items {
			resourceType, resourceID, ok := resourceSummaryFields(item)
			if !ok {
				logger.Warn("skipping malformed search result", "region", region, "resource_type", resourceType, "resource_id", resourceID)
				result.AddSkipped(1)
				continue
			}

			jobs <- updateJob{
				item:         item,
				region:       region,
				resourceType: resourceType,
				resourceID:   resourceID,
			}
		}
	}

	close(jobs)
	updateWG.Wait()

	result.SetMessage("LicenseExcluded invoke complete")

	return &result
}

func (c *RegionalClient) Search(ctx context.Context, region string) (resourcesearch.ResourceSummaryCollection, error) {
	result := resourcesearch.ResourceSummaryCollection{Items: make([]resourcesearch.ResourceSummary, 0)}

	for _, q := range []string{query, dbQuery, integrationQuery, clusterQuery} {
		page := ""
		for {
			request := resourcesearch.SearchResourcesRequest{
				SearchDetails: resourcesearch.StructuredSearchDetails{
					Query: common.String(q),
				},
				Limit: common.Int(searchLimit),
			}
			if page != "" {
				request.Page = common.String(page)
			}

			response, err := c.SearchClient.SearchResources(ctx, request)
			if err != nil {
				return result, fmt.Errorf("search resources in %s: %w", region, err)
			}

			result.Items = append(result.Items, response.Items...)

			if response.OpcNextPage == nil || *response.OpcNextPage == "" {
				break
			}
			page = *response.OpcNextPage
		}
	}

	return result, nil
}

func (c RegionalClient) handleResource(ctx context.Context, item resourcesearch.ResourceSummary, region, resourceType, resourceID string) (bool, error) {
	switch resourceType {
	case "DbSystem":
		return c.handleDbSystem(ctx, item, region, resourceID)
	case "AutonomousDatabase":
		return c.handleAutonomousDatabase(ctx, item, region, resourceID)
	case "AnalyticsInstance":
		return c.handleAnalyticsInstance(ctx, item, region, resourceID)
	case "IntegrationInstance":
		return c.handleIntegrationInstance(ctx, item, region, resourceID)
	case "CloudVmCluster":
		return c.handleCloudVmCluster(ctx, item, region, resourceID)
	case "VmCluster":
		return c.handleVmCluster(ctx, item, region, resourceID)
	case "AutonomousVmCluster":
		return c.handleAutonomousVmCluster(ctx, item, region, resourceID)
	case "CloudAutonomousVmCluster":
		return c.handleCloudAutonomousVmCluster(ctx, item, region, resourceID)
	default:
		logger.Warn("unsupported resource type", "region", region, "resource_type", resourceType, "resource_id", resourceID)
		return false, nil
	}
}

func (c RegionalClient) handleAutonomousDatabase(ctx context.Context, adb resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling autonomous database", "region", region, "resource_id", resourceID)

	response, err := c.DatabaseClient.GetAutonomousDatabase(ctx, database.GetAutonomousDatabaseRequest{
		AutonomousDatabaseId: adb.Identifier,
	})
	if err != nil {
		return false, fmt.Errorf("get autonomous database: %w", err)
	}
	if response.AutonomousDatabase.IsFreeTier != nil && *response.AutonomousDatabase.IsFreeTier {
		logger.Debug("skipping free-tier autonomous database", "region", region, "resource_id", resourceID)
		return false, nil
	}

	if response.AutonomousDatabase.LicenseModel != database.AutonomousDatabaseLicenseModelLicenseIncluded {
		return false, nil
	}

	logger.Info("changing autonomous database license to BYOL", "region", region, "resource_id", resourceID)
	resp, err := c.DatabaseClient.UpdateAutonomousDatabase(ctx, database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: adb.Identifier,
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			LicenseModel:    database.UpdateAutonomousDatabaseDetailsLicenseModelBringYourOwnLicense,
			DatabaseEdition: database.AutonomousDatabaseSummaryDatabaseEditionEnterpriseEdition,
		},
	})
	if err != nil {
		return false, fmt.Errorf("update autonomous database: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusOK) {
		return false, fmt.Errorf("update autonomous database returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated autonomous database", "region", region, "resource_id", resourceID)
	return true, nil
}

func (c RegionalClient) handleDbSystem(ctx context.Context, db resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling DB system", "region", region, "resource_id", resourceID)
	logger.Info("changing DB system license to BYOL", "region", region, "resource_id", resourceID)

	resp, err := c.DatabaseClient.UpdateDbSystem(ctx, database.UpdateDbSystemRequest{
		DbSystemId: db.Identifier,
		UpdateDbSystemDetails: database.UpdateDbSystemDetails{
			LicenseModel: database.UpdateDbSystemDetailsLicenseModelBringYourOwnLicense,
		},
	})
	if err != nil {
		return false, fmt.Errorf("update DB system: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusOK) {
		return false, fmt.Errorf("update DB system returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated DB system", "region", region, "resource_id", resourceID)
	return true, nil
}

func (c RegionalClient) handleAnalyticsInstance(ctx context.Context, ai resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling analytics instance", "region", region, "resource_id", resourceID)

	response, err := c.AnalyticsClient.GetAnalyticsInstance(ctx, analytics.GetAnalyticsInstanceRequest{
		AnalyticsInstanceId: ai.Identifier,
	})
	if err != nil {
		return false, fmt.Errorf("get analytics instance: %w", err)
	}
	if response.AnalyticsInstance.LicenseType != analytics.LicenseTypeLicenseIncluded {
		return false, nil
	}

	logger.Info("changing analytics instance license to BYOL", "region", region, "resource_id", resourceID)
	resp, err := c.AnalyticsClient.UpdateAnalyticsInstance(ctx, analytics.UpdateAnalyticsInstanceRequest{
		AnalyticsInstanceId: ai.Identifier,
		UpdateAnalyticsInstanceDetails: analytics.UpdateAnalyticsInstanceDetails{
			LicenseType: analytics.LicenseTypeBringYourOwnLicense,
		},
	})
	if err != nil {
		return false, fmt.Errorf("update analytics instance: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusOK) {
		return false, fmt.Errorf("update analytics instance returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated analytics instance", "region", region, "resource_id", resourceID)
	return true, nil
}

func (c RegionalClient) handleIntegrationInstance(ctx context.Context, i resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling integration instance", "region", region, "resource_id", resourceID)
	logger.Info("changing integration instance license to BYOL", "region", region, "resource_id", resourceID)

	resp, err := c.IntegrationClient.UpdateIntegrationInstance(ctx, integration.UpdateIntegrationInstanceRequest{
		IntegrationInstanceId: i.Identifier,
		UpdateIntegrationInstanceDetails: integration.UpdateIntegrationInstanceDetails{
			IsByol: common.Bool(true),
		},
	})
	if err != nil {
		return false, fmt.Errorf("update integration instance: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusAccepted) {
		return false, fmt.Errorf("update integration instance returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated integration instance", "region", region, "resource_id", resourceID)
	return true, nil
}

func (c RegionalClient) handleCloudVmCluster(ctx context.Context, cluster resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling cloud VM cluster", "region", region, "resource_id", resourceID)

	response, err := c.DatabaseClient.GetCloudVmCluster(ctx, database.GetCloudVmClusterRequest{
		CloudVmClusterId: cluster.Identifier,
	})
	if err != nil {
		return false, fmt.Errorf("get cloud VM cluster: %w", err)
	}
	if response.CloudVmCluster.LicenseModel != database.CloudVmClusterLicenseModelLicenseIncluded {
		return false, nil
	}

	logger.Info("changing cloud VM cluster license to BYOL", "region", region, "resource_id", resourceID)
	resp, err := c.DatabaseClient.UpdateCloudVmCluster(ctx, database.UpdateCloudVmClusterRequest{
		CloudVmClusterId: cluster.Identifier,
		UpdateCloudVmClusterDetails: database.UpdateCloudVmClusterDetails{
			LicenseModel: database.UpdateCloudVmClusterDetailsLicenseModelBringYourOwnLicense,
		},
	})
	if err != nil {
		return false, fmt.Errorf("update cloud VM cluster: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusOK, http.StatusAccepted) {
		return false, fmt.Errorf("update cloud VM cluster returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated cloud VM cluster", "region", region, "resource_id", resourceID)
	return true, nil
}

func (c RegionalClient) handleVmCluster(ctx context.Context, cluster resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling VM cluster", "region", region, "resource_id", resourceID)

	response, err := c.DatabaseClient.GetVmCluster(ctx, database.GetVmClusterRequest{
		VmClusterId: cluster.Identifier,
	})
	if err != nil {
		return false, fmt.Errorf("get VM cluster: %w", err)
	}
	if response.VmCluster.LicenseModel != database.VmClusterLicenseModelLicenseIncluded {
		return false, nil
	}

	logger.Info("changing VM cluster license to BYOL", "region", region, "resource_id", resourceID)
	resp, err := c.DatabaseClient.UpdateVmCluster(ctx, database.UpdateVmClusterRequest{
		VmClusterId: cluster.Identifier,
		UpdateVmClusterDetails: database.UpdateVmClusterDetails{
			LicenseModel: database.UpdateVmClusterDetailsLicenseModelBringYourOwnLicense,
		},
	})
	if err != nil {
		return false, fmt.Errorf("update VM cluster: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusOK, http.StatusAccepted) {
		return false, fmt.Errorf("update VM cluster returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated VM cluster", "region", region, "resource_id", resourceID)
	return true, nil
}

func (c RegionalClient) handleAutonomousVmCluster(ctx context.Context, cluster resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling autonomous VM cluster", "region", region, "resource_id", resourceID)

	response, err := c.DatabaseClient.GetAutonomousVmCluster(ctx, database.GetAutonomousVmClusterRequest{
		AutonomousVmClusterId: cluster.Identifier,
	})
	if err != nil {
		return false, fmt.Errorf("get autonomous VM cluster: %w", err)
	}
	if response.AutonomousVmCluster.LicenseModel != database.AutonomousVmClusterLicenseModelLicenseIncluded {
		return false, nil
	}

	logger.Info("changing autonomous VM cluster license to BYOL", "region", region, "resource_id", resourceID)
	resp, err := c.DatabaseClient.UpdateAutonomousVmCluster(ctx, database.UpdateAutonomousVmClusterRequest{
		AutonomousVmClusterId: cluster.Identifier,
		UpdateAutonomousVmClusterDetails: database.UpdateAutonomousVmClusterDetails{
			LicenseModel: database.UpdateAutonomousVmClusterDetailsLicenseModelBringYourOwnLicense,
		},
	})
	if err != nil {
		return false, fmt.Errorf("update autonomous VM cluster: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusOK, http.StatusAccepted) {
		return false, fmt.Errorf("update autonomous VM cluster returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated autonomous VM cluster", "region", region, "resource_id", resourceID)
	return true, nil
}

func (c RegionalClient) handleCloudAutonomousVmCluster(ctx context.Context, cluster resourcesearch.ResourceSummary, region, resourceID string) (bool, error) {
	logger.Debug("handling cloud autonomous VM cluster", "region", region, "resource_id", resourceID)

	response, err := c.DatabaseClient.GetCloudAutonomousVmCluster(ctx, database.GetCloudAutonomousVmClusterRequest{
		CloudAutonomousVmClusterId: cluster.Identifier,
	})
	if err != nil {
		return false, fmt.Errorf("get cloud autonomous VM cluster: %w", err)
	}
	if response.CloudAutonomousVmCluster.LicenseModel != database.CloudAutonomousVmClusterLicenseModelLicenseIncluded {
		return false, nil
	}

	logger.Info("changing cloud autonomous VM cluster license to BYOL", "region", region, "resource_id", resourceID)
	resp, err := c.DatabaseClient.UpdateCloudAutonomousVmCluster(ctx, database.UpdateCloudAutonomousVmClusterRequest{
		CloudAutonomousVmClusterId: cluster.Identifier,
		UpdateCloudAutonomousVmClusterDetails: database.UpdateCloudAutonomousVmClusterDetails{
			LicenseModel: database.UpdateCloudAutonomousVmClusterDetailsLicenseModelBringYourOwnLicense,
		},
	})
	if err != nil {
		return false, fmt.Errorf("update cloud autonomous VM cluster: %w", err)
	}
	if !statusOK(resp.RawResponse, http.StatusOK, http.StatusAccepted) {
		return false, fmt.Errorf("update cloud autonomous VM cluster returned %s", responseStatus(resp.RawResponse))
	}

	logger.Debug("updated cloud autonomous VM cluster", "region", region, "resource_id", resourceID)
	return true, nil
}

func resourceSummaryFields(item resourcesearch.ResourceSummary) (string, string, bool) {
	resourceType := ""
	resourceID := ""
	if item.ResourceType != nil {
		resourceType = *item.ResourceType
	}
	if item.Identifier != nil {
		resourceID = *item.Identifier
	}

	return resourceType, resourceID, resourceType != "" && resourceID != ""
}

func statusOK(resp *http.Response, expected ...int) bool {
	if resp == nil {
		return false
	}
	for _, status := range expected {
		if resp.StatusCode == status {
			return true
		}
	}
	return false
}

func responseStatus(resp *http.Response) string {
	if resp == nil {
		return "nil response"
	}
	return resp.Status
}

func sortedKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K {
	return slices.Sorted(maps.Keys(m))
}
