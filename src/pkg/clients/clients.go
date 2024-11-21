package clients

import (
	"context"
	"func/pkg/logging"
	"os"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/analytics"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"
)

const query string = `query dbsystem, autonomousdatabase, analyticsinstance resources
where lifeCycleState = 'RUNNING' || lifeCycleState = 'STOPPED' || lifeCycleState = 'AVAILABLE' 
|| lifeCycleState = 'ACTIVE' || lifeCycleState = 'INACTIVE'`

var logger logging.Lumberjack = logging.NewLogger(os.Getenv("LOG_LEVEL"))

type RegionalClient struct {
	AnalyticsClient analytics.AnalyticsClient
	DatabaseClient  database.DatabaseClient
	SearchClient    resourcesearch.ResourceSearchClient
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

	return RegionalClient{
		AnalyticsClient: ac,
		DatabaseClient:  dc,
		SearchClient:    sc,
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
func (c ClientBundle) ProcessCollection() {
	logger.Debug("Processing resource collection")

	var wg sync.WaitGroup

	sc := SearchCollection{
		Items: make(map[string]resourcesearch.ResourceSummaryCollection),
	}

	logger.Debugf("Searching for resources with query: %s\n", query)
	for region, client := range c {
		logger.Info("Searching in", region)
		wg.Add(1)
		go func(client RegionalClient, region string) {
			defer wg.Done()
			rc := client.Search()
			logger.Infof("Found %v resources in %v\n", len(rc.Items), region)

			sc.Lock()
			defer sc.Unlock()

			sc.Items[region] = rc
			//sc.Items = append(r.Items, rc.Items...)
		}(client, region)
	}
	logger.Debugf("Assembled resource collection: %v", sc.Items)

	wg.Wait()

	for region, items := range sc.Items {
		for _, item := range items.Items {
			switch *item.ResourceType {
			case "DbSystem":
				wg.Add(1)
				go func(item resourcesearch.ResourceSummary, region string) {
					defer wg.Done()
					c[region].handleDbSystem(item)
				}(item, region)
			case "AutonomousDatabase":
				wg.Add(1)
				go func(item resourcesearch.ResourceSummary, region string) {
					defer wg.Done()
					c[region].handleAutonomousDatabase(item)
				}(item, region)
			case "AnalyticsInstance":
				wg.Add(1)
				go func(item resourcesearch.ResourceSummary, region string) {
					defer wg.Done()
					c[region].handleAnalyticsInstance(item)
				}(item, region)
			default:
				logger.Warn("Error: No supported type", *item.ResourceType)
			}
		}
	}

	wg.Wait()
}

// Search uses the search client to find resources.
func (c *RegionalClient) Search() resourcesearch.ResourceSummaryCollection {

	details := resourcesearch.StructuredSearchDetails{
		Query: common.String(query),
	}

	request := resourcesearch.SearchResourcesRequest{
		SearchDetails: details,
		Limit:         common.Int(1000),
	}

	response, err := c.SearchClient.SearchResources(context.Background(), request)
	logErrAndContinue(err)

	return response.ResourceSummaryCollection
}

func (c RegionalClient) handleAutonomousDatabase(adb resourcesearch.ResourceSummary) {
	logger.Debugf("Handling Autonomous Database %v\n", *adb.Identifier)
	request := database.GetAutonomousDatabaseRequest{
		AutonomousDatabaseId: adb.Identifier,
	}
	response, err := c.DatabaseClient.GetAutonomousDatabase(context.Background(), request)
	if err != nil {
		logger.Errorf("Error handling Autonomous Database %v, %v\n", *adb.Identifier, err)
		return
	}

	// Exclusive to Autonomous Database
	if *response.AutonomousDatabase.IsFreeTier {
		logger.Debugf("%v is free tier, skipping", *adb.Identifier)
		return
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
			logger.Infof("Updated Autonomous Database %v\n", *adb.Identifier)
		}
	}
}

func (c RegionalClient) handleDbSystem(db resourcesearch.ResourceSummary) {
	logger.Debugf("Handling DBSystem %v\n", *db.Identifier)
	request := database.GetDbSystemRequest{
		DbSystemId: db.Identifier,
	}

	response, err := c.DatabaseClient.GetDbSystem(context.Background(), request)
	if err != nil {
		logger.Errorf("Error handling DBSystem %v, %v\n", *db.Identifier, err)
		return
	}

	if response.DbSystem.LicenseModel == database.DbSystemLicenseModelLicenseIncluded {
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
			logger.Infof("Updated DBSystem %v\n", *db.Identifier)
		}
	}
}

func (c RegionalClient) handleAnalyticsInstance(ai resourcesearch.ResourceSummary) {
	logger.Debugf("Handling Analytics Instance %v\n", *ai.Identifier)
	request := analytics.GetAnalyticsInstanceRequest{
		AnalyticsInstanceId: ai.Identifier,
	}

	response, err := c.AnalyticsClient.GetAnalyticsInstance(context.Background(), request)
	if err != nil {
		logger.Errorf("Error handling AnalyticsInstance %v, %v\n", *ai.Identifier, err)
		return
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
			logger.Infof("Updated AnalyticsInstance %v\n", *ai.Identifier)
		}
	}
}

// Logs error if not nil and moves on without stopping execution
func logErrAndContinue(err error) {
	if err != nil {
		logger.Error(err)
	}
}
