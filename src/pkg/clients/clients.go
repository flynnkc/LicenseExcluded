package clients

import (
	"context"
	"encoding/json"
	"func/pkg/logging"
	"func/pkg/results"
	"os"
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
)

var logger logging.Lumberjack = logging.NewLogger(os.Getenv("LOG_LEVEL"))

type RegionalClient struct {
	AnalyticsClient   analytics.AnalyticsClient
	DatabaseClient    database.DatabaseClient
	SearchClient      resourcesearch.ResourceSearchClient
	IntegrationClient integration.IntegrationInstanceClient
}

type ClientBundle map[string]RegionalClient

type SearchCollection struct {
	Items map[string]resourcesearch.ResourceSummaryCollection
	sync.Mutex
}

func NewRegionalClient(p common.ConfigurationProvider) RegionalClient {

	ac, err := analytics.NewAnalyticsClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	dc, err := database.NewDatabaseClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	sc, err := resourcesearch.NewResourceSearchClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	ic, err := integration.NewIntegrationInstanceClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	return RegionalClient{
		AnalyticsClient:   ac,
		DatabaseClient:    dc,
		SearchClient:      sc,
		IntegrationClient: ic,
	}
}

// NewClientBundle takes a ConfigurationProvider and a list of regions, creates clients for each region,
// and returns them as a bundle.
func NewClientBundle(p common.ConfigurationProvider, regions []identity.RegionSubscription) ClientBundle {
	logger.Debug("Making new client bundles")
	logger.Debug("Regions:", regions)

	cb := make(ClientBundle)

	for _, r := range regions {
		newProvider, err := auth.ResourcePrincipalConfigurationProviderForRegion(common.StringToRegion(*r.RegionName))
		if err != nil {
			logger.Errorf("Problem with client bundle provider: %v\n", err)
			continue
		}

		cb[*r.RegionName] = NewRegionalClient(newProvider)
	}

	logger.Debug("Client bundles assembled")
	return cb
}

// ProcessCollection fans out on returned resources
func (c ClientBundle) ProcessCollection() *results.Result {
	logger.Debug("Processing resource collection")

	var wg sync.WaitGroup

	sc := SearchCollection{
		Items: make(map[string]resourcesearch.ResourceSummaryCollection),
	}
	result := results.NewResult()

	logger.Debugf("Searching %v regions with queries:\n\t%s\n\t%s\n\t%s\n",
		len(c), query, dbQuery, integrationQuery)
	for region, client := range c {
		logger.Debug("Searching in", region)
		wg.Add(1)
		go func(client RegionalClient, region string, result *results.Result) {
			defer wg.Done()
			rc := client.Search()
			logger.Infof("Found %v resources in %v\n", len(rc.Items), region)

			sc.Lock()
			defer sc.Unlock()

			sc.Items[region] = rc
			result.AddItemsFound(len(rc.Items))
		}(client, region, &result)
	}

	wg.Wait()

	if logger.Level == logging.DEBUG {
		b, err := json.Marshal(&sc)
		if err != nil {
			logger.Error("Error marshalling search collection into json")
		}
		logger.Debugf("Items returned: %v", string(b))
	}

	for region, items := range sc.Items {
		for _, item := range items.Items {
			switch *item.ResourceType {
			case "DbSystem":
				wg.Add(1)
				go func(item resourcesearch.ResourceSummary, region string, result *results.Result) {
					defer wg.Done()
					if c[region].handleDbSystem(item) {
						result.AddChanges(1)
					}
				}(item, region, &result)

			case "AutonomousDatabase":
				wg.Add(1)
				go func(item resourcesearch.ResourceSummary, region string, result *results.Result) {
					defer wg.Done()
					if c[region].handleAutonomousDatabase(item) {
						result.AddChanges(1)
					}
				}(item, region, &result)

			case "AnalyticsInstance":
				wg.Add(1)
				go func(item resourcesearch.ResourceSummary, region string, result *results.Result) {
					defer wg.Done()
					if c[region].handleAnalyticsInstance(item) {
						result.AddChanges(1)
					}
				}(item, region, &result)

			case "IntegrationInstance":
				wg.Add(1)
				go func(item resourcesearch.ResourceSummary, region string, result *results.Result) {
					defer wg.Done()
					if c[region].handleIntegrationInstance(item) {
						result.AddChanges(1)
					}
				}(item, region, &result)

			default:
				logger.Warn("Error: No supported type", *item.ResourceType)
			}
		}
	}

	wg.Wait()

	result.SetMessage("LicenseExcluded invoke complete")

	return &result
}

// Search uses the search client to find resources.
func (c *RegionalClient) Search() resourcesearch.ResourceSummaryCollection {

	result := resourcesearch.ResourceSummaryCollection{Items: make([]resourcesearch.ResourceSummary, 0)}

	for _, q := range []string{query, dbQuery, integrationQuery} {

		details := resourcesearch.StructuredSearchDetails{
			Query: common.String(q),
		}

		request := resourcesearch.SearchResourcesRequest{
			SearchDetails: details,
			Limit:         common.Int(1000),
		}

		response, err := c.SearchClient.SearchResources(context.Background(), request)
		logErrAndContinue(err)

		result.Items = append(result.Items, response.Items...)
	}

	return result
}

// handlers check for license type and change if incorrect license is found. Returns
// true to signal that a change was made.
func (c RegionalClient) handleAutonomousDatabase(adb resourcesearch.ResourceSummary) bool {
	logger.Debugf("Handling Autonomous Database %v\n", *adb.Identifier)
	request := database.GetAutonomousDatabaseRequest{
		AutonomousDatabaseId: adb.Identifier,
	}
	response, err := c.DatabaseClient.GetAutonomousDatabase(context.Background(), request)
	if err != nil {
		logger.Errorf("Error handling Autonomous Database %v, %v\n", *adb.Identifier, err)
		return false
	}

	// Exclusive to Autonomous Database
	if *response.AutonomousDatabase.IsFreeTier {
		logger.Debugf("%v is free tier, skipping", *adb.Identifier)
		return false
	}

	if response.AutonomousDatabase.LicenseModel == database.AutonomousDatabaseLicenseModelLicenseIncluded {
		logger.Infof("%v - Changing from License Included to BYOL\n", *adb.Identifier)
		req := database.UpdateAutonomousDatabaseRequest{
			AutonomousDatabaseId: adb.Identifier,
			UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
				LicenseModel:    database.UpdateAutonomousDatabaseDetailsLicenseModelBringYourOwnLicense,
				DatabaseEdition: database.AutonomousDatabaseSummaryDatabaseEditionEnterpriseEdition,
			},
		}

		resp, err := c.DatabaseClient.UpdateAutonomousDatabase(context.Background(), req)
		if err != nil {
			logger.Errorf("Error updating Autonomous Database %v, %v\n", *adb.Identifier, err)
		} else if resp.RawResponse.StatusCode != 200 {
			logger.Errorf("Non-200 status code returned %v - %v\n", resp.RawResponse.Status, *adb.Identifier)
		} else {
			logger.Debugf("Updated Autonomous Database %v\n", *adb.Identifier)
			return true
		}
	}
	return false
}

// Handle DbSystem updates
func (c RegionalClient) handleDbSystem(db resourcesearch.ResourceSummary) bool {
	logger.Debugf("Handling DBSystem %v\n", *db.Identifier)

	// Don't need to check for license because query only returns license included systems
	logger.Infof("%v - Changing from License Included to BYOL\n", *db.Identifier)
	req := database.UpdateDbSystemRequest{
		DbSystemId: db.Identifier,
		UpdateDbSystemDetails: database.UpdateDbSystemDetails{
			LicenseModel: database.UpdateDbSystemDetailsLicenseModelBringYourOwnLicense,
		},
	}

	resp, err := c.DatabaseClient.UpdateDbSystem(context.Background(), req)
	if err != nil {
		logger.Errorf("Error updating DBSystem %v, %v\n", *db.Identifier, err)
	} else if resp.RawResponse.StatusCode != 200 {
		logger.Errorf("Non-200 status code returned %v - %v\n", resp.RawResponse.Status, *db.Identifier)
	} else {
		logger.Debugf("Updated DBSystem %v\n", *db.Identifier)
		return true
	}

	return false
}

func (c RegionalClient) handleAnalyticsInstance(ai resourcesearch.ResourceSummary) bool {
	logger.Debugf("Handling Analytics Instance %v\n", *ai.Identifier)
	request := analytics.GetAnalyticsInstanceRequest{
		AnalyticsInstanceId: ai.Identifier,
	}

	response, err := c.AnalyticsClient.GetAnalyticsInstance(context.Background(), request)
	if err != nil {
		logger.Errorf("Error handling AnalyticsInstance %v, %v\n", *ai.Identifier, err)
		return false
	}

	if response.AnalyticsInstance.LicenseType == analytics.LicenseTypeLicenseIncluded {
		logger.Infof("%v - Changing from License Included to BYOL\n", *ai.Identifier)
		req := analytics.UpdateAnalyticsInstanceRequest{
			AnalyticsInstanceId: ai.Identifier,
			UpdateAnalyticsInstanceDetails: analytics.UpdateAnalyticsInstanceDetails{
				LicenseType: analytics.LicenseTypeBringYourOwnLicense,
			},
		}

		resp, err := c.AnalyticsClient.UpdateAnalyticsInstance(context.Background(), req)
		if err != nil {
			logger.Errorf("Error updating AnalyticsInstance %v, %v\n", *ai.Identifier, err)
		} else if resp.RawResponse.StatusCode != 200 {
			logger.Errorf("Non-200 status code returned %v - %v\n", resp.RawResponse.Status, *ai.Identifier)
		} else {
			logger.Debugf("Updated AnalyticsInstance %v\n", *ai.Identifier)
			return true
		}
	}
	return false
}

func (c RegionalClient) handleIntegrationInstance(i resourcesearch.ResourceSummary) bool {
	logger.Debugf("Handling Integration Instance %v\n", *i.Identifier)

	// Don't need to check for byol as search only returns license included instances
	req := integration.UpdateIntegrationInstanceRequest{
		IntegrationInstanceId: i.Identifier,
		UpdateIntegrationInstanceDetails: integration.UpdateIntegrationInstanceDetails{
			IsByol: common.Bool(true),
		},
	}

	resp, err := c.IntegrationClient.UpdateIntegrationInstance(context.Background(), req)
	if err != nil {
		logger.Errorf("Error updating IntegrationInstance %v, %v\n", *i.Identifier, err)
	} else if resp.RawResponse.StatusCode != 202 {
		logger.Errorf("Non-20X status code returned %v - %v\n", resp.RawResponse.Status, *i.Identifier)
	} else {
		logger.Debugf("Updated IntegrationInstance %v\n", *i.Identifier)
		return true
	}

	return false
}

// Logs error if not nil and moves on without stopping execution
func logErrAndContinue(err error) {
	if err != nil {
		logger.Error(err)
	}
}
