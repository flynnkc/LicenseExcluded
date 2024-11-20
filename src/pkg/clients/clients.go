package clients

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/analytics"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"
)

const query string = `query dbsystem, autonomousdatabase, analyticsinstance resources
where lifeCycleState = 'RUNNING' || lifeCycleState = 'STOPPED' || lifeCycleState = 'AVAILABLE'`

type RegionalClient struct {
	AnalyticsClient analytics.AnalyticsClient
	DatabaseClient  database.DatabaseClient
	//IdentityClient  identity.IdentityClient
	SearchClient resourcesearch.ResourceSearchClient
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

	//ic, err := identity.NewIdentityClientWithConfigurationProvider(p)
	//logErrAndContinue(err)

	sc, err := resourcesearch.NewResourceSearchClientWithConfigurationProvider(p)
	logErrAndContinue(err)

	return RegionalClient{
		AnalyticsClient: ac,
		DatabaseClient:  dc,
		//IdentityClient:  ic,
		SearchClient: sc,
	}
}

// NewClientBundle takes a ConfigurationProvider and a list of regions, creates clients for each region,
// and returns them as a bundle.
func NewClientBundle(p common.ConfigurationProvider, regions []identity.RegionSubscription) ClientBundle {
	cb := make(ClientBundle)

	for _, r := range regions {
		newProvider, err := constructConfigurationProvider(*r.RegionName, p)
		if err != nil {
			fmt.Println("Error creating provider:", err)
			continue
		}

		cb[*r.RegionName] = NewRegionalClient(*newProvider)
	}

	return cb
}

// ProcessCollection fans out on returned resources
func (c ClientBundle) ProcessCollection() {

	var wg sync.WaitGroup

	sc := SearchCollection{
		Items: make(map[string]resourcesearch.ResourceSummaryCollection),
	}

	for region, client := range c {
		wg.Add(1)
		go func(client RegionalClient, region string) {
			defer wg.Done()
			rc := client.Search()

			sc.Lock()
			defer sc.Unlock()

			sc.Items[region] = rc
			//sc.Items = append(r.Items, rc.Items...)
		}(client, region)
	}

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
					fmt.Println("AnalyticsInstance", region, item)
				}(item, region)
			default:
				fmt.Println("Error: No supported type", *item.ResourceType)
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

func constructConfigurationProvider(region string, provider common.ConfigurationProvider) (*common.ConfigurationProvider, error) {
	tenancy, err := provider.TenancyOCID()
	if err != nil {
		return nil, err
	}

	user, err := provider.UserOCID()
	if err != nil {
		return nil, err
	}

	fingerprint, err := provider.KeyFingerprint()
	if err != nil {
		return nil, err
	}

	passphrase := ""

	/*
		// Get key into string
		key, err := provider.PrivateRSAKey()
		if err != nil {
			return nil, err
		}

		pk := string(x509.MarshalPKCS1PrivateKey(key))
	*/

	pb, err := os.ReadFile(os.Getenv("KC_KEY"))
	logErrAndContinue(err)
	pk := string(pb)

	provider = common.NewRawConfigurationProvider(tenancy, user, region, fingerprint, pk, &passphrase)

	return &provider, nil
}

func (c RegionalClient) handleAutonomousDatabase(adb resourcesearch.ResourceSummary) {
	fmt.Printf("Handling Autonomous Database %v\n", *adb.Identifier)
	request := database.GetAutonomousDatabaseRequest{
		AutonomousDatabaseId: adb.Identifier,
	}
	response, err := c.DatabaseClient.GetAutonomousDatabase(context.Background(), request)
	if err != nil {
		fmt.Printf("Error handling Autonomous Database %v, %v\n", *adb.Identifier, err)
		return
	}

	// Exclusive to Autonomous Database
	if *response.AutonomousDatabase.IsFreeTier {
		return
	}

	if response.AutonomousDatabase.LicenseModel == database.AutonomousDatabaseLicenseModelLicenseIncluded {
		fmt.Printf("%v - Changing from License Included to BYOL\n", *adb.Identifier)
		req := database.UpdateAutonomousDatabaseRequest{
			AutonomousDatabaseId: adb.Identifier,
			UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
				LicenseModel:    database.UpdateAutonomousDatabaseDetailsLicenseModelBringYourOwnLicense,
				DatabaseEdition: database.AutonomousDatabaseSummaryDatabaseEditionEnterpriseEdition,
			},
		}

		resp, err := c.DatabaseClient.UpdateAutonomousDatabase(context.Background(), req)
		if err != nil {
			fmt.Printf("Error updating Autonomous Database %v, %v\n", *adb.Identifier, err)
		} else if resp.RawResponse.StatusCode != 200 {
			fmt.Printf("Non-200 status code returned %v - %v\n", resp.RawResponse.StatusCode, *adb.Identifier)
		} else {
			fmt.Printf("Updated Autonomous Database %v\n", *adb.Identifier)
		}
	}
}

func (c RegionalClient) handleDbSystem(db resourcesearch.ResourceSummary) {
	fmt.Printf("Handling DBSystem %v\n", *db.Identifier)
	request := database.GetDbSystemRequest{
		DbSystemId: db.Identifier,
	}

	response, err := c.DatabaseClient.GetDbSystem(context.Background(), request)
	if err != nil {
		fmt.Printf("Error handling DBSystem %v, %v\n", *db.Identifier, err)
		return
	}

	if response.DbSystem.LicenseModel == database.DbSystemLicenseModelLicenseIncluded {
		req := database.UpdateDbSystemRequest{
			DbSystemId: db.Identifier,
			UpdateDbSystemDetails: database.UpdateDbSystemDetails{
				LicenseModel: database.UpdateDbSystemDetailsLicenseModelBringYourOwnLicense,
			},
		}

		resp, err := c.DatabaseClient.UpdateDbSystem(context.Background(), req)
		if err != nil {
			fmt.Printf("Error updating DBSystem %v, %v\n", *db.Identifier, err)
		} else if resp.RawResponse.StatusCode != 200 {
			fmt.Printf("Non-200 status code returned %v - %v\n", resp.RawResponse.StatusCode, *db.Identifier)
		} else {
			fmt.Printf("Updated DBSystem %v\n", *db.Identifier)
		}
	}
}

// Logs error if not nil and moves on without stopping execution
func logErrAndContinue(err error) {
	if err != nil {
		fmt.Println("Error:", err)
	}
}
